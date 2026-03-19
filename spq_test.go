package super_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/sio/arrowio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
	"github.com/brimdata/super/ztest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSPQ(t *testing.T) {
	t.Parallel()

	dirs, err := findZTests()
	require.NoError(t, err)

	t.Run("boomerang", func(t *testing.T) {
		t.Parallel()
		data, err := loadZTestInputsAndOutputs(dirs)
		require.NoError(t, err)
		runAllBoomerangs(t, "arrows", data)
		runAllBoomerangs(t, "csup", data)
		runAllBoomerangs(t, "parquet", data)
		runAllBoomerangs(t, "sup", data)
		runAllBoomerangs(t, "jsup", data)
	})

	t.Run("fusion", func(t *testing.T) {
		t.Parallel()
		data, err := loadZTestInputsAndOutputs(dirs)
		require.NoError(t, err)
		runAll(t, "arrows", data, fusion)
		runAll(t, "csup", data, fusion)
		runAll(t, "parquet", data, fusion)
		runAll(t, "sup", data, fusion)
		runAll(t, "jsup", data, fusion)
	})

	for d := range dirs {
		t.Run(filepath.ToSlash(d), func(t *testing.T) {
			t.Parallel()
			ztest.Run(t, d)
		})
	}
}

func findZTests() (map[string]struct{}, error) {
	dirs := map[string]struct{}{}
	pattern := fmt.Sprintf(`.*ztests\%c.*\.yaml$`, filepath.Separator)
	re := regexp.MustCompile(pattern)
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".yaml") && re.MatchString(path) {
			dirs[filepath.Dir(path)] = struct{}{}
		}
		return nil
	})
	return dirs, err
}

func loadZTestInputsAndOutputs(ztestDirs map[string]struct{}) (map[string]string, error) {
	out := map[string]string{}
	for dir := range ztestDirs {
		bundles, err := ztest.Load(dir)
		if err != nil {
			return nil, err
		}
		for _, b := range bundles {
			if b.Test == nil {
				continue
			}
			testName := b.FileName + "/" + strconv.Itoa(b.Test.Line)
			if i := b.Test.Input; i != nil && isValid(*i) {
				out[testName+"/input"] = *i
			}
			if o := b.Test.Output; isValid(o) {
				out[testName+"/output"] = o
			}
			for _, i := range b.Test.Inputs {
				if i.Data != nil && isValid(*i.Data) {
					out[testName+"/inputs/"+i.Name] = *i.Data
				}
			}
			for _, o := range b.Test.Outputs {
				if o.Data != nil && isValid(*o.Data) {
					out[testName+"/outputs/"+o.Name] = *o.Data
				}
			}
		}
	}
	return out, nil
}

// isValid returns true if and only if s can be read fully without error by
// anyio and contains at least one value.
func isValid(s string) bool {
	zrc, err := anyio.NewReader(super.NewContext(), strings.NewReader(s), anyio.ReaderOpts{})
	if err != nil {
		return false
	}
	defer zrc.Close()
	var foundValue bool
	for {
		val, err := zrc.Read()
		if err != nil {
			return false
		}
		if val == nil {
			return foundValue
		}
		foundValue = true
	}
}

func runAllBoomerangs(t *testing.T, format string, data map[string]string) {
	t.Run(format, func(t *testing.T) {
		t.Parallel()
		for name, data := range data {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				fusion(t, format, data)
			})
		}
	})
}

func runAll(t *testing.T, format string, data map[string]string, f func(t *testing.T, format, data string)) {
	t.Run(format, func(t *testing.T) {
		t.Parallel()
		for name, data := range data {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				f(t, format, data)
			})
		}
	})
}

func runOneBoomerang(t *testing.T, format, data string) {
	// Create an auto-detecting reader for data.
	sctx := super.NewContext()
	dataReadCloser, err := anyio.NewReader(sctx, strings.NewReader(data), anyio.ReaderOpts{})
	require.NoError(t, err)
	defer dataReadCloser.Close()

	dataReader := sio.Reader(dataReadCloser)
	if format == "parquet" {
		// Fuse for formats that require uniform values.
		q, err := query(t.Context(), sctx, "fuse", dataReadCloser)
		require.NoError(t, err)
		defer q.Pull(true)
		dataReader = sbuf.PullerReader(sbuf.NewMaterializer(q))
	}

	// Copy from dataReader to baseline as format.
	var baseline bytes.Buffer
	writerOpts := anyio.WriterOpts{Format: format}
	baselineWriter, err := anyio.NewWriter(sio.NopCloser(&baseline), writerOpts)
	if err == nil {
		err = vio.Copy(baselineWriter, sbuf.NewDematerializer(sctx, sbuf.NewPuller(dataReader)))
		require.NoError(t, baselineWriter.Close())
	}
	if err != nil {
		if errors.Is(err, arrowio.ErrMultipleTypes) ||
			errors.Is(err, arrowio.ErrNotRecord) ||
			errors.Is(err, arrowio.ErrUnsupportedType) {
			t.Skipf("skipping due to expected error: %s", err)
		}
		t.Fatalf("unexpected error writing %s baseline: %s", format, err)
	}

	// Create a reader for baseline.
	baselineReader, err := anyio.NewReader(super.NewContext(), bytes.NewReader(baseline.Bytes()), anyio.ReaderOpts{
		Format: format,
		BSUP: bsupio.ReaderOpts{
			Validate: true,
		},
	})
	require.NoError(t, err)
	defer baselineReader.Close()

	// Copy from baselineReader to boomerang as format.
	var boomerang bytes.Buffer
	boomerangWriter, err := anyio.NewWriter(sio.NopCloser(&boomerang), writerOpts)
	require.NoError(t, err)
	assert.NoError(t, vio.Copy(boomerangWriter, sbuf.NewDematerializer(sctx, sbuf.NewPuller(baselineReader))))
	require.NoError(t, boomerangWriter.Close())

	require.Equal(t, baseline.String(), boomerang.String(), "baseline and boomerang differ")
}

func fusion(t *testing.T, format, data string) {
	// Create an auto-detecting reader for data.
	sctx := super.NewContext()
	dataReader, err := anyio.NewReader(sctx, strings.NewReader(data), anyio.ReaderOpts{})
	require.NoError(t, err)
	defer dataReader.Close()

	// Write data to baseline as format.
	baseline, err := readAll(sbuf.NewDematerializer(sctx, sbuf.NewPuller(dataReader)), format)
	if err != nil {
		if errors.Is(err, arrowio.ErrMultipleTypes) ||
			errors.Is(err, arrowio.ErrNotRecord) ||
			errors.Is(err, arrowio.ErrUnsupportedType) {
			t.Skipf("skipping due to expected error: %s", err)
		}
		t.Fatalf("unexpected error writing %s baseline: %s", format, err)
	}

	// Write data to fusion as format after fusing and defusing.
	dataReader, err = anyio.NewReader(sctx, strings.NewReader(data), anyio.ReaderOpts{})
	require.NoError(t, err)
	defer dataReader.Close()

	q, err := query(t.Context(), sctx, "fuse | defuse(this)", dataReader)
	require.NoError(t, err)
	defer q.Pull(true)
	fusion, err := readAll(q, format)
	require.NoError(t, err)

	require.Equal(t, string(baseline), string(fusion), "baseline and fusion differ")
}

func query(ctx context.Context, sctx *super.Context, spq string, r sio.Reader) (vio.Puller, error) {
	ast, err := parser.ParseText(spq)
	if err != nil {
		return nil, err
	}
	e := exec.NewEnvironment(nil, nil)
	//e.Runtime = exec.RuntimeVAM
	rctx := runtime.NewContext(ctx, sctx)
	q, err := compiler.NewCompilerWithEnv(e).NewQuery(rctx, ast, []sio.Reader{r}, 0)
	if err != nil {
		return nil, err
	}
	return q, nil
}

func readAll(src vio.Puller, outputFormat string) ([]byte, error) {
	var b bytes.Buffer
	w, err := anyio.NewWriter(sio.NopCloser(&b), anyio.WriterOpts{Format: outputFormat})
	if err != nil {
		return nil, err
	}
	for {
		var vec vector.Any
		vec, err = src.Pull(false)
		if l, ok := vec.(*vector.Labeled); ok {
			vec = l.Any
		}
		if vec == nil || err != nil {
			break
		}
		err = w.Push(vec)
		if err != nil {
			break
		}

	}
	//err = vio.Copy(w, src)
	err2 := w.Close()
	return b.Bytes(), errors.Join(err, err2)
}
