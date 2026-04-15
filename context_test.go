package super_test

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextLookupTypeNamedErrors(t *testing.T) {
	sctx := super.NewContext()

	_, err := sctx.LookupTypeNamed("\xff", super.TypeNull)
	assert.EqualError(t, err, `bad type name "\xff": invalid UTF-8`)

	_, err = sctx.LookupTypeNamed("null", super.TypeNull)
	assert.EqualError(t, err, `named type collides with primitive type: null`)
}

func TestContextLookupTypeNamedAndLookupTypeDef(t *testing.T) {
	sctx := super.NewContext()

	assert.Nil(t, sctx.LookupByName("x"))

	named1, err := sctx.LookupTypeNamed("x", super.TypeNull)
	require.NoError(t, err)
	assert.Same(t, named1, sctx.LookupByName("x"))
}
