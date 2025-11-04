package ztest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldSkip(t *testing.T) {
	assert.Equal(t, "in-process test on script run", (&ZTest{SPQ: "x"}).ShouldSkip("y"))
	assert.Equal(t, "script test on in-process run", (&ZTest{Script: "x"}).ShouldSkip(""))
	assert.Equal(t, "reason", (&ZTest{Skip: "reason"}).ShouldSkip(""))
	assert.Equal(t, `tag "x" does not match ZTEST_TAG=""`, (&ZTest{Tag: "x"}).ShouldSkip(""))
}

func TestRunScript(t *testing.T) {
	t.Run("outputs", func(t *testing.T) {
		testDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "testdirfile"), []byte("testdirfile\n"), 0644))
		strptr := func(s string) *string { return &s }
		err := (&ZTest{
			Script: `
				echo stdout
				echo stderr >&2
				touch empty
				echo notempty > notempty
				echo regexp > regexp
				echo testdirfile > testdirfile
				echo testdirfile > testdirfile2
				`,
			Outputs: []File{
				{Name: "stdout", Data: strptr("stdout\n")},
				{Name: "stderr", Data: strptr("stderr\n")},
				{Name: "empty", Data: strptr("")},
				{Name: "notempty", Data: strptr("notempty\n")},
				{Name: "regexp", Re: "^re"},
				{Name: "testdirfile"},
				{Name: "testdirfile2", Source: "testdirfile"},
			},
		}).RunScript(t.Context(), "", testDir, t.TempDir)
		assert.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		err := (&ZTest{
			Script:  "echo 1; echo 2 >&2; exit 3",
			Outputs: []File{},
		}).RunScript(t.Context(), "", "", func() string { return "" })
		assert.EqualError(t, err, "script failed: exit status 3\n=== stdout ===\n1\n=== stderr ===\n2\n")
	})
}
