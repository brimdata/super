// Package ztest runs formulaic tests ("ztests") that can be (1) run in-process
// with the compiled-ini zq code base, (2) run as a sub-process using the zq
// executable build artifact, or (3) run as a bash script running a sequence
// of arbitrary shell commands invoking any of the build artifacts.  The
// first two cases comprise the "Zed test style" and the last case
// comprises the "script test style".  Case (1) is easier to debug by
// simply running "go test" compared replicating the test using "go run".
// Script-style tests don't have this convenience.
//
// In the Zed style, ztest runs a Zed program on an input and checks
// for an expected output.
//
// A Zed-style test is defined in a YAML file.
//
//	zed: count()
//
//	input: |
//	  #0:record[i:int64]
//	  0:[1;]
//	  0:[2;]
//
//	output: |
//	  #0:record[count:uint64]
//	  0:[2;]
//
// Input format is detected automatically and can be anything recognized by
// "zq -i auto" (including optional gzip compression).  Output format defaults
// to SUP but can be set to anything accepted by "zq -f".
//
//	zed: count()
//
//	input: |
//	  #0:record[i:int64]
//	  0:[1;]
//	  0:[2;]
//
//	output-flags: -f table
//
//	output: |
//	  count
//	  2
//
// Alternatively, tests can be configured to run as shell scripts.
// In this style of test, arbitrary bash scripts can run chaining together
// any of the tools in cmd/ in addition to zq.  Scripts are executed by "bash -e
// -o pipefail", and a nonzero shell exit code causes a test failure, so any failed
// command generally results in a test failure.  Here, the yaml sets up a collection
// of input files and stdin, the script runs, and the test driver compares expected
// output files, stdout, and stderr with data in the yaml spec.  In this case,
// instead of specifying, "zed", "input", "output", you specify the yaml arrays
// "inputs" and "outputs" --- where each array element defines a file, stdin,
// stdout, or stderr --- and a "script" that specifies a multi-line yaml string
// defining the script, e.g.,
//
// inputs:
//   - name: in1.sup
//     data: |
//     #0:record[i:int64]
//     0:[1;]
//   - name: stdin
//     data: |
//     #0:record[i:int64]
//     0:[2;]
//
// script: |
//
//	zq -o out.sup in1.sup -
//	zq -o count.sup "count()" out.sup
//
// outputs:
//   - name: out.sup
//     data: |
//     #0:record[i:int64]
//     0:[1;]
//     0:[2;]
//   - name: count.sup
//     data: |
//     #0:record[count:uint64]
//     0:[2;]
//
// Each input and output has a name.  For inputs, a file (source)
// or inline data (data) may be specified.
// If no data is specified, then a file of the same name as the
// name field is looked for in the same directory as the yaml file.
// The source spec is a file path relative to the directory of the
// yaml file.  For outputs, expected output is defined in the same
// fashion as the inputs though you can also specify a "regexp" string
// instead of expected data.  If an output is named "stdout" or "stderr"
// then the actual output is taken from the stdout or stderr of the
// the shell script.
//
// Ztest YAML files for a package should reside in a subdirectory named
// testdata/ztest.
//
//	pkg/
//	  pkg.go
//	  pkg_test.go
//	  testdata/
//	    ztest/
//	      test-1.yaml
//	      test-2.yaml
//	      ...
//
// Name YAML files descriptively since each ztest runs as a subtest
// named for the file that defines it.
//
// pkg_test.go should contain a Go test named TestZTest that calls Run.
//
//	func TestZTest(t *testing.T) { ztest.Run(t, "testdata/ztest") }
//
// If the ZTEST_PATH environment variable is unset or empty and the test
// is not a script test, Run runs ztests in the current process and skips
// the script tests.  Otherwise, Run runs each ztest in a separate process
// using the zq executable in the directories specified by ZTEST_PATH.
//
// Tests of either style can be skipped by setting the skip field to a non-empty
// string.  A message containing the string will be written to the test log.
package ztest

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/inputflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/vam"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"
)

func ShellPath() string {
	return os.Getenv("ZTEST_PATH")
}

type Bundle struct {
	TestName string
	FileName string
	Test     *ZTest
	Error    error
}

func Load(dirname string) ([]Bundle, error) {
	var bundles []Bundle
	fileinfos, err := os.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	for _, fi := range fileinfos {
		filename := fi.Name()
		const dotyaml = ".yaml"
		if !strings.HasSuffix(filename, dotyaml) {
			continue
		}
		testname := strings.TrimSuffix(filename, dotyaml)
		filename = filepath.Join(dirname, filename)
		zt, err := FromYAMLFile(filename)
		bundles = append(bundles, Bundle{testname, filename, zt, err})
	}
	return bundles, nil
}

// Run runs the ztests in the directory named dirname.  For each file f.yaml in
// the directory, Run calls FromYAMLFile to load a ztest and then runs it in
// subtest named f.  path is a command search path like the
// PATH environment variable.
func Run(t *testing.T, dirname string) {
	shellPath := ShellPath()
	bundles, err := Load(dirname)
	if err != nil {
		t.Fatal(err)
	}
	for _, bundle := range bundles {
		b := bundle
		t.Run(b.TestName, func(t *testing.T) {
			t.Parallel()
			if b.Error != nil {
				t.Fatalf("%s: %s", b.FileName, b.Error)
			}
			b.Test.Run(t, shellPath, b.FileName)
		})
	}
}

type File struct {
	// Name is the name of the file with respect to the directoy in which
	// the test script runs.  For inputs, if no data source is specified,
	// then name is also the name of a data file in the diectory containing
	// the yaml test file, which is copied to the test script directory.
	// Name can also be stdio (for inputs) or stdout or stderr (for outputs).
	Name string `yaml:"name"`
	// Data and Source represent the different ways file data can
	// be defined for this file.  Data is a string turned into the contents
	// of the file. Source is a string representing
	// the pathname of a file the repo that is read to comprise the data.
	Data   *string `yaml:"data,omitempty"`
	Source string  `yaml:"source,omitempty"`
	// Re is a regular expression describing the contents of the file,
	// which is only applicable to output files.
	Re string `yaml:"regexp,omitempty"`
}

func (f *File) check() error {
	cnt := 0
	if f.Data != nil {
		cnt++
	}
	if f.Source != "" {
		cnt++
	}
	if cnt > 1 {
		return fmt.Errorf("%s: must specify at most one of data or source", f.Name)
	}
	return nil
}

func (f *File) load(dir string) ([]byte, *regexp.Regexp, error) {
	if f.Data != nil {
		return []byte(*f.Data), nil, nil
	}
	if f.Source != "" {
		b, err := os.ReadFile(filepath.Join(dir, f.Source))
		return b, nil, err
	}
	if f.Re != "" {
		re, err := regexp.Compile(f.Re)
		return nil, re, err
	}
	b, err := os.ReadFile(filepath.Join(dir, f.Name))
	if err == nil {
		return b, nil, nil
	}
	if os.IsNotExist(err) {
		err = fmt.Errorf("%s: no data source", f.Name)
	}
	return nil, nil, err
}

// ZTest defines a ztest.
type ZTest struct {
	Skip string `yaml:"skip,omitempty"`
	Tag  string `yaml:"tag,omitempty"`

	// For Zed-style tests.
	Zed         string `yaml:"zed,omitempty"`
	Input       string `yaml:"input,omitempty"`
	InputFlags  string `yaml:"input-flags,omitempty"`
	Output      string `yaml:"output,omitempty"`
	OutputFlags string `yaml:"output-flags,omitempty"`
	Error       string `yaml:"error,omitempty"`
	Vector      bool   `yaml:"vector"`

	// For script-style tests.
	Script  string   `yaml:"script,omitempty"`
	Inputs  []File   `yaml:"inputs,omitempty"`
	Outputs []File   `yaml:"outputs,omitempty"`
	Env     []string `yaml:"env,omitempty"`
}

func (z *ZTest) check() error {
	if z.Script != "" {
		if z.Outputs == nil {
			return errors.New("outputs field missing in a sh test")
		}
		for _, f := range z.Inputs {
			if err := f.check(); err != nil {
				return err
			}
			if f.Re != "" {
				return fmt.Errorf("%s: cannot use regexp in an input", f.Name)
			}
		}
		for _, f := range z.Outputs {
			if err := f.check(); err != nil {
				return err
			}
		}
	} else if z.Zed == "" {
		return errors.New("either a zed field or script field must be present")
	}
	return nil
}

// FromYAMLFile loads a ZTest from the YAML file named filename.
func FromYAMLFile(filename string) (*ZTest, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	d := yaml.NewDecoder(bytes.NewReader(buf))
	d.KnownFields(true)
	var z ZTest
	if err := d.Decode(&z); err != nil {
		return nil, err
	}
	var v interface{}
	if d.Decode(&v) != io.EOF {
		return nil, errors.New("found multiple YAML documents or garbage after first document")
	}
	return &z, nil
}

func (z *ZTest) ShouldSkip(path string) string {
	switch {
	case z.Script != "" && path == "":
		return "script test on in-process run"
	case z.Zed != "" && path != "":
		return "in-process test on script run"
	case z.Skip != "":
		return z.Skip
	case z.Tag != "" && z.Tag != os.Getenv("ZTEST_TAG"):
		return fmt.Sprintf("tag %q does not match ZTEST_TAG=%q", z.Tag, os.Getenv("ZTEST_TAG"))
	}
	return ""
}

func (z *ZTest) RunScript(shellPath, testDir string, tempDir func() string) error {
	if err := z.check(); err != nil {
		return fmt.Errorf("bad yaml format: %w", err)
	}
	serr := runsh(shellPath, testDir, tempDir(), z)
	if !z.Vector {
		return serr
	}
	if serr != nil {
		serr = fmt.Errorf("=== sequence ===\n%w", serr)
	}
	verr := runsh(shellPath, testDir, tempDir(), z, "SUPER_VAM=1")
	if verr != nil {
		verr = fmt.Errorf("=== vector ===\n%w", verr)
	}
	return errors.Join(serr, verr)
}

func (z *ZTest) RunInternal() error {
	if err := z.check(); err != nil {
		return fmt.Errorf("bad yaml format: %w", err)
	}
	outputFlags := append([]string{"-f=sup", "-pretty=0"}, strings.Fields(z.OutputFlags)...)
	inputFlags := strings.Fields(z.InputFlags)
	if z.Vector {
		verr := z.diffInternal(runvec(z.Zed, z.Input, outputFlags, inputFlags))
		if verr != nil {
			verr = fmt.Errorf("=== vector ===\n%w", verr)
		}
		serr := z.diffInternal(runzq(z.Zed, z.Input, outputFlags, inputFlags))
		if serr != nil {
			serr = fmt.Errorf("=== sequence ===\n%w", serr)
		}
		return errors.Join(verr, serr)
	}
	return z.diffInternal(runzq(z.Zed, z.Input, outputFlags, inputFlags))
}

func (z *ZTest) diffInternal(out string, err error) error {
	var outDiffErr, errDiffErr error
	if z.Output != out {
		outDiffErr = diffErr("output", z.Output, out)
	}
	var errStr string
	if err != nil {
		// Append newline if err doesn't end with one.
		errStr = strings.TrimSuffix(err.Error(), "\n") + "\n"
	}
	if z.Error != errStr {
		errDiffErr = diffErr("error", z.Error, errStr)
	}
	return errors.Join(outDiffErr, errDiffErr)
}

func (z *ZTest) Run(t *testing.T, path, filename string) {
	if msg := z.ShouldSkip(path); msg != "" {
		t.Skip("skipping test:", msg)
	}
	var err error
	if z.Script != "" {
		err = z.RunScript(path, filepath.Dir(filename), t.TempDir)
	} else {
		err = z.RunInternal()
	}
	if err != nil {
		t.Fatalf("%s: %s", filename, err)
	}
}

func diffErr(name, expected, actual string) error {
	if !utf8.ValidString(expected) {
		expected = hex.Dump([]byte(expected))
		actual = hex.Dump([]byte(actual))
	}
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(expected),
		FromFile: "expected",
		B:        difflib.SplitLines(actual),
		ToFile:   "actual",
		Context:  5,
	})
	if err != nil {
		panic("ztest: " + err.Error())
	}
	return fmt.Errorf("expected and actual %s differ:\n%s", name, diff)
}

func runsh(path, testDir, tempDir string, zt *ZTest, extraEnv ...string) error {
	var stdin io.Reader
	for _, f := range zt.Inputs {
		b, _, err := f.load(testDir)
		if err != nil {
			return err
		}
		if f.Name == "stdin" {
			stdin = bytes.NewReader(b)
			continue
		}
		if err := os.WriteFile(filepath.Join(tempDir, f.Name), b, 0644); err != nil {
			return err
		}
	}
	stdout, stderr, err := RunShell(tempDir, path, zt.Script, stdin, zt.Env, extraEnv)
	if err != nil {
		return fmt.Errorf("script failed: %w\n=== stdout ===\n%s=== stderr ===\n%s",
			err, stdout, stderr)
	}
	for _, f := range zt.Outputs {
		var actual string
		switch f.Name {
		case "stdout":
			actual = stdout
		case "stderr":
			actual = stderr
		default:
			b, err := os.ReadFile(filepath.Join(tempDir, f.Name))
			if err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
			actual = string(b)
		}
		expected, expectedRE, err := f.load(testDir)
		if err != nil {
			return err
		}
		if expected != nil && string(expected) != actual {
			return diffErr(f.Name, string(expected), actual)
		}
		if expectedRE != nil && !expectedRE.MatchString(actual) {
			return fmt.Errorf("%s: regexp %q does not match %q", f.Name, expectedRE, actual)
		}
	}
	return nil
}

// runzq runs zedProgram over input and returns the output.  input
// may be in any format recognized by "zq -i auto" and may be gzip-compressed.
// outputFlags may contain any flags accepted by cli/outputflags.Flags.
func runzq(zedProgram, input string, outputFlags []string, inputFlags []string) (string, error) {
	ast, err := parser.ParseQuery(zedProgram)
	if err != nil {
		return "", err
	}
	zctx := super.NewContext()
	zrc, err := newInputReader(zctx, input, inputFlags)
	if err != nil {
		return "", err
	}
	defer zrc.Close()
	var outflags outputflags.Flags
	var flags flag.FlagSet
	outflags.SetFlags(&flags)
	if err := flags.Parse(outputFlags); err != nil {
		return "", err
	}
	if err := outflags.Init(); err != nil {
		return "", err
	}
	var outbuf bytes.Buffer
	zw, err := anyio.NewWriter(zio.NopCloser(&outbuf), outflags.Options())
	if err != nil {
		return "", err
	}
	q, err := runtime.CompileQuery(context.Background(), zctx, compiler.NewCompiler(nil), ast, []zio.Reader{zrc})
	if err != nil {
		zw.Close()
		return "", err
	}
	defer q.Pull(true)
	err = zbuf.CopyPuller(zw, q)
	if err2 := zw.Close(); err == nil {
		err = err2
	}
	return outbuf.String(), err
}

func runvec(zedProgram string, input string, outputFlags, inputFlags []string) (string, error) {
	var flags flag.FlagSet
	var outflags outputflags.Flags
	outflags.SetFlags(&flags)
	if err := flags.Parse(outputFlags); err != nil {
		return "", err
	}
	if err := outflags.Init(); err != nil {
		return "", err
	}
	zctx := super.NewContext()
	zrc, err := newInputReader(zctx, input, inputFlags)
	if err != nil {
		return "", err
	}
	d := vam.NewDematerializer(zbuf.NewPuller(zrc))
	rctx := runtime.NewContext(context.Background(), zctx)
	puller, err := compiler.VectorCompile(rctx, zedProgram, d)
	if err != nil {
		return "", err
	}
	var outbuf bytes.Buffer
	zw, err := anyio.NewWriter(zio.NopCloser(&outbuf), outflags.Options())
	if err != nil {
		return "", err
	}
	err = zbuf.CopyPuller(zw, puller)
	if err2 := zw.Close(); err == nil {
		err = err2
	}
	return outbuf.String(), err
}

func newInputReader(zctx *super.Context, input string, flags []string) (zio.ReadCloser, error) {
	var inflags inputflags.Flags
	var fs flag.FlagSet
	inflags.SetFlags(&fs, true)
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	r, err := anyio.GzipReader(strings.NewReader(input))
	if err != nil {
		return nil, err
	}
	return anyio.NewReaderWithOpts(zctx, r, inflags.Options())
}
