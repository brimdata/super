package super_test

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextLookupTypeNamedErrors(t *testing.T) {
	sctx := super.NewContext()

	_, err := sctx.LookupTypeNamed("\xff", super.TypeNull)
	assert.EqualError(t, err, `bad type name "\xff": invalid UTF-8`)

	_, err = sctx.LookupTypeNamed("null", super.TypeNull)
	assert.EqualError(t, err, `bad type name "null": primitive type name`)
}

func TestContextLookupTypeNamedAndLookupTypeDef(t *testing.T) {
	sctx := super.NewContext()

	assert.Nil(t, sctx.LookupTypeDef("x"))

	named1, err := sctx.LookupTypeNamed("x", super.TypeNull)
	require.NoError(t, err)
	assert.Same(t, named1, sctx.LookupTypeDef("x"))

	named2, err := sctx.LookupTypeNamed("x", super.TypeInt8)
	require.NoError(t, err)
	assert.Same(t, named2, sctx.LookupTypeDef("x"))

	named3, err := sctx.LookupTypeNamed("x", super.TypeNull)
	require.NoError(t, err)
	assert.Same(t, named3, sctx.LookupTypeDef("x"))
	assert.Same(t, named3, named1)
}

func TestContextTranslateTypeNameConflictUnion(t *testing.T) {
	// This test confirms that a union with complicated type renaming is properly
	// decoded.  There was a bug where child typedefs would override the
	// top level typedef in TranslateType so foo in the value below had
	// two of the same union type instead of the two it should have had.
	sctx := super.NewContext()
	val := sup.MustParseValue(sctx, `[{x:{y:63}}::=foo,{x:{abcdef:{x:{y:127}}::foo}}::=foo]`)
	foreign := super.NewContext()
	twin, err := foreign.TranslateType(val.Type())
	require.NoError(t, err)
	union := twin.(*super.TypeArray).Type.(*super.TypeUnion)
	assert.Equal(t, `foo={x:{abcdef:foo={x:{y:int64}}}}`, sup.String(union.Types[0]))
	assert.Equal(t, `foo={x:{y:int64}}`, sup.String(union.Types[1]))
}
