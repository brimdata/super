package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURIWinVolume(t *testing.T) {
	expected := `c:\test\folder`
	uri, err := ParseURI(expected)
	require.NoError(t, err)
	assert.Equal(t, "file:///c:/test/folder", uri.String())
	assert.Equal(t, expected, uri.Filepath())

}

func TestURIWinFileScheme(t *testing.T) {
	uri, err := ParseURI("file:///c:/test/folder")
	require.NoError(t, err)
	assert.Equal(t, `c:\test\folder`, uri.Filepath())
}

func TestURIWinRelative(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	expected := filepath.Join(cwd, "a", "c")
	uri, err := ParseURI(`a\b\..\c`)
	require.NoError(t, err)
	assert.Equal(t, expected, uri.Filepath())
}

// github.com/superdb/zui#1284
func TestURIWinUNCPath(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{name: "BackwardSlash", value: `\\34.82.284.241\foo`},
		{name: "ForwardSlash", value: `//34.82.284.241/foo`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uri, err := ParseURI(c.value)
			require.NoError(t, err)
			assert.Equal(t, "file:////34.82.284.241/foo", uri.String())
			assert.Equal(t, `\\34.82.284.241\foo`, uri.Filepath())
		})
	}
}
