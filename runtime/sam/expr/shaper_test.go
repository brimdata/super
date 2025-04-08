package expr

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBestUnionTag(t *testing.T) {
	u8 := super.TypeUint8
	sctx := super.NewContext()
	u8named1, err := sctx.LookupTypeNamed("u8named1", u8)
	require.NoError(t, err)
	u8named2, err := sctx.LookupTypeNamed("u8named2", u8)
	require.NoError(t, err)
	u8named3, err := sctx.LookupTypeNamed("u8named3", u8)
	require.NoError(t, err)

	assert.Equal(t, -1, bestUnionTag(u8, nil))
	assert.Equal(t, -1, bestUnionTag(u8, u8))
	assert.Equal(t, -1, bestUnionTag(super.TypeUint16, sctx.LookupTypeUnion([]super.Type{u8})))

	test := func(expected, needle super.Type, haystack []super.Type) {
		t.Helper()
		union := sctx.LookupTypeUnion(haystack)
		typ, err := union.Type(bestUnionTag(needle, union))
		if assert.NoError(t, err) {
			assert.Equal(t, expected, typ)
		}

	}

	// Needle is in haystack.
	test(u8, u8, []super.Type{u8, u8named1, u8named2})
	test(u8, u8, []super.Type{u8named2, u8named1, u8})
	test(u8, u8, []super.Type{u8named1, u8, u8named2})
	test(u8named2, u8named2, []super.Type{u8, u8named1, u8named2})
	test(u8named2, u8named2, []super.Type{u8named2, u8named1, u8})
	test(u8named2, u8named2, []super.Type{u8, u8named2, u8named1})

	// Underlying type of needle is in haystack.
	test(u8, u8named1, []super.Type{u8, u8named2, u8named3})
	test(u8, u8named1, []super.Type{u8named3, u8named2, u8})
	test(u8, u8named1, []super.Type{u8named2, u8, u8named3})

	// Type compatible with needle is in haystack.
	test(u8named1, u8, []super.Type{u8named1, u8named2, u8named3})
	test(u8named2, u8named1, []super.Type{u8named3, u8named2})
}
