package reglob_test

import (
	"testing"

	"github.com/brimdata/zq/reglob"
	"github.com/stretchr/testify/require"
)

func TestReglob(t *testing.T) {
	expected := "^S.*$"
	actual := reglob.Reglob("S*")
	require.Equal(t, expected, actual)
}

func Test_SingleStar(t *testing.T) {
	expected := "^.*$"
	actual := reglob.Reglob("*")
	require.Equal(t, expected, actual)
}

func TestBackslashes(t *testing.T) {
	pattern := `\xaa\*\x55`
	expected := `^\xaa\*\x55$`
	require.Equal(t, expected, reglob.Reglob(pattern))
}
