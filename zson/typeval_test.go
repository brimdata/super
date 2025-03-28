package zson_test

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zson"
	"github.com/stretchr/testify/require"
)

func TestTypeValue(t *testing.T) {
	const s = "{A:{B:int64},C:int32}"
	typ, err := zson.ParseType(super.NewContext(), s)
	require.NoError(t, err)
	tv := super.NewContext().LookupTypeValue(typ)
	require.Exactly(t, s, zson.FormatTypeValue(tv.Bytes()))
}

func TestTypeValueCrossContext(t *testing.T) {
	const s = "{A:{B:int64},C:int32}"
	typ, err := zson.ParseType(super.NewContext(), s)
	require.NoError(t, err)
	tv := super.NewContext().LookupTypeValue(typ)
	require.Exactly(t, s, zson.FormatTypeValue(tv.Bytes()))
}
