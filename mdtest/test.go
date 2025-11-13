package mdtest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Test represents a single test in a Markdown file.
type Test struct {
	Command   string
	Dir       string
	Expected  string
	Fails     bool
	Head      bool
	Line      int
	GoExample string

	// For SPQ tests
	Input string
	SPQ   string
}

// Run runs the test, returning nil on success.
func (t *Test) Run() error {
	if t.GoExample != "" {
		return t.vetGoExample()
	}
	var c *exec.Cmd
	if t.SPQ != "" {
		c = exec.Command("super", "-s", "-c", t.SPQ)
		if s := t.Input; strings.TrimSpace(s) != "" {
			c.Args = append(c.Args, "-")
			c.Stdin = strings.NewReader(s)
		}
	} else {
		c = exec.Command("bash", "-e", "-o", "pipefail")
		c.Dir = t.Dir
		c.Stdin = strings.NewReader(t.Command)
	}
	outBytes, err := c.CombinedOutput()
	out := string(outBytes)
	if t.Fails {
		if errors.As(err, new(*exec.ExitError)) {
			err = nil
		} else if err == nil {
			err = errors.New("command succeeded unexpectedly")
		}
	}
	if err != nil {
		if out != "" {
			return fmt.Errorf("%w\noutput:\n%s", err, out)
		}
		return err
	}
	if t.Head && len(out) > len(t.Expected) {
		out = out[:len(t.Expected)]
	}
	if out != t.Expected {
		diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(t.Expected),
			FromFile: "expected",
			B:        difflib.SplitLines(out),
			ToFile:   "actual",
			Context:  5,
		})
		if err != nil {
			return err
		}
		return fmt.Errorf("expected and actual output differ:\n%s", diff)
	}
	return nil
}

func (t *Test) vetGoExample() error {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(t.GoExample), 0666); err != nil {
		return err
	}
	_, err = exec.Command("go", "vet", path).Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("could not vet go example: %s", string(exitErr.Stderr))
	}
	return err
}
