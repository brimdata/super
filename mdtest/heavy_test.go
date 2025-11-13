//go:build heavy

package mdtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkdownExamples(t *testing.T) {
	require.NoError(t, os.Chdir(".."))
	files, err := Load()
	require.NoError(t, err)
	require.NotZero(t, len(files))
	for _, f := range files {
		t.Run(filepath.ToSlash(f.Path), func(t *testing.T) {
			t.Parallel()
			f.Run(t)
		})
	}
}
