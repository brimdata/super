package super_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/sio/arrowio"
	"github.com/brimdata/super/sio/bsupio"
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
			if i := b.Test.Input; i != nil && isValid(*i) {
				out[b.FileName+"/input"] = *i
			}
			if o := b.Test.Output; isValid(o) {
				out[b.FileName+"/output"] = o
			}
			for _, i := range b.Test.Inputs {
				if i.Data != nil && isValid(*i.Data) {
					out[b.FileName+"/inputs/"+i.Name] = *i.Data
				}
			}
			for _, o := range b.Test.Outputs {
				if o.Data != nil && isValid(*o.Data) {
					out[b.FileName+"/outputs/"+o.Name] = *o.Data
				}
			}
		}
	}
	return out, nil
}

// isValid returns true if and only if s can be read fully without error by
// anyio and contains at least one value.
func isValid(s string) bool {
	zrc, err := anyio.NewReader(super.NewContext(), strings.NewReader(s))
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
				runOneBoomerang(t, format, data)
			})
		}
	})
}

func runOneBoomerang(t *testing.T, format, data string) {
	// Create an auto-detecting reader for data.
	sctx := super.NewContext()
	dataReadCloser, err := anyio.NewReader(sctx, strings.NewReader(data))
	require.NoError(t, err)
	defer dataReadCloser.Close()

	dataReader := sio.Reader(dataReadCloser)
	if format == "parquet" {
		// Fuse for formats that require uniform values.
		ast, err := parser.ParseQuery("fuse")
		require.NoError(t, err)
		rctx := runtime.NewContext(t.Context(), sctx)
		q, err := compiler.NewCompiler(nil).NewQuery(rctx, ast, []sio.Reader{dataReadCloser}, 0)
		require.NoError(t, err)
		defer q.Pull(true)
		dataReader = sbuf.PullerReader(q)
	}

	// Copy from dataReader to baseline as format.
	var baseline bytes.Buffer
	writerOpts := anyio.WriterOpts{Format: format}
	baselineWriter, err := anyio.NewWriter(sio.NopCloser(&baseline), writerOpts)
	if err == nil {
		err = sio.Copy(baselineWriter, dataReader)
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
	baselineReader, err := anyio.NewReaderWithOpts(super.NewContext(), bytes.NewReader(baseline.Bytes()), anyio.ReaderOpts{
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
	assert.NoError(t, sio.Copy(boomerangWriter, baselineReader))
	require.NoError(t, boomerangWriter.Close())

	require.Equal(t, baseline.String(), boomerang.String(), "baseline and boomerang differ")
}
