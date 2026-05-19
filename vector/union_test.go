package vector

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/stretchr/testify/require"
)

func TestNewUnionVerifyPanics(t *testing.T) {
	sctx := super.NewContext()
	intVec := NewInt(super.TypeInt64, nil)
	utyp, _ := sctx.LookupTypeUnion([]super.Type{super.TypeInt64, super.TypeString})
	require.PanicsWithValue(t, "NewUnion: must have one vector for each type", func() {
		NewUnion(utyp, nil, []Any{intVec})
	})
	require.PanicsWithValue(t, "NewUnion: multiple vectors with the same type int64", func() {
		NewUnion(utyp, nil, []Any{intVec, intVec})
	})
	ipVec := NewIP(nil)
	require.PanicsWithValue(t, "NewUnion: type ip not a member of union int64|string", func() {
		NewUnion(utyp, nil, []Any{intVec, ipVec})
	})
}

func TestNewUnionFromRLEVerifyPanics(t *testing.T) {
	sctx := super.NewContext()
	intVec := NewInt(super.TypeInt64, nil)
	utyp, _ := sctx.LookupTypeUnion([]super.Type{super.TypeInt64, super.TypeString})
	require.PanicsWithValue(t, "NewUnion: must have one vector for each type", func() {
		NewUnionFromRLE(utyp, nil, []Any{intVec})
	})
	require.PanicsWithValue(t, "NewUnion: multiple vectors with the same type int64", func() {
		NewUnionFromRLE(utyp, nil, []Any{intVec, intVec})
	})
	ipVec := NewIP(nil)
	require.PanicsWithValue(t, "NewUnion: type ip not a member of union int64|string", func() {
		NewUnionFromRLE(utyp, nil, []Any{intVec, ipVec})
	})
}
