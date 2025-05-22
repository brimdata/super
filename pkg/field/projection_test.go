package field

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjection(t *testing.T) {
	cases := [][]Path{
		{
			Path{"a", "b"},
		},
		{
			Path{"a", "b"},
			Path{"c", "d"},
		},
		{
			Path{"a", "b"},
			Path{"a", "c"},
		},
		{
			Path{"a", "b", "c"},
			Path{"a", "b", "d"},
		},
	}
	for _, c := range cases {
		proj := NewProjection(c)
		assert.Equal(t, c, proj.Paths(), "projection: %#v", proj)
	}
}
