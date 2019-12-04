package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueries(t *testing.T) {
	t.Parallel()
	for _, d := range internals {
		t.Run(d.Name, func(t *testing.T) {
			results, err := d.Run()
			require.NoError(t, err)
			assert.Exactly(t, d.Expected, results, "Wrong query results")
		})
	}
	for _, cmd := range commands {
		t.Run(cmd.Name, func(t *testing.T) {
			results, err := cmd.Run()
			require.NoError(t, err)
			assert.Exactly(t, cmd.Expected, results, "Wrong query results")
		})
	}
}
