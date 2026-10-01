package sup_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/superdb/super"
	"github.com/superdb/super/sup"
)

func TestTypeValue(t *testing.T) {
	const s = "{A:{B:int64},C:int32}"
	const expected = "<{A:{B:int64},C:int32}>"

	sctx := super.NewContext()
	typ, err := sup.ParseType(sctx, s)
	require.NoError(t, err)
	tv := sctx.LookupTypeValue(typ)
	require.Exactly(t, expected, sup.FormatTypeValue(tv.Bytes()))
}

func TestTypeValueCrossContext(t *testing.T) {
	const s = "{A:{B:int64},C:int32}"
	const expected = "<{A:{B:int64},C:int32}>"
	sctx := super.NewContext()
	typ, err := sup.ParseType(sctx, s)
	require.NoError(t, err)
	other := super.NewContext()
	otherType, err := other.TranslateType(typ)
	require.NoError(t, err)
	tv := other.LookupTypeValue(otherType)
	require.Exactly(t, expected, sup.FormatTypeValue(tv.Bytes()))
}
