package zson_test

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zson"
	"github.com/stretchr/testify/require"
)

func TestTypeValue(t *testing.T) {
	const s = "{A:{B:int64},C:int32}"
	sctx := super.NewContext()
	typ, err := zson.ParseType(sctx, s)
	require.NoError(t, err)
	tv := sctx.LookupTypeValue(typ)
	require.Exactly(t, s, zson.FormatTypeValue(tv.Bytes()))
}

func TestTypeValueCrossContext(t *testing.T) {
	const s = "{A:{B:int64},C:int32}"
	sctx := super.NewContext()
	typ, err := zson.ParseType(sctx, s)
	require.NoError(t, err)
	other := super.NewContext()
	otherType, err := other.TranslateType(typ)
	require.NoError(t, err)
	tv := other.LookupTypeValue(otherType)
	require.Exactly(t, s, zson.FormatTypeValue(tv.Bytes()))
}
