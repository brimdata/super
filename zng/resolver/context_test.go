package resolver_test

import (
	"testing"

	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextAddColumns(t *testing.T) {
	ctx := resolver.NewContext()
	d, err := ctx.LookupTypeRecord([]zng.Column{zng.NewColumn("s1", zng.TypeString)})
	require.NoError(t, err)
	b := zng.NewBuilder(d)
	r, err := b.Parse("S1")
	require.NoError(t, err)
	cols := []zng.Column{zng.NewColumn("ts", zng.TypeTime), zng.NewColumn("s2", zng.TypeString)}
	ts, _ := nano.Parse([]byte("123.456"))
	r, err = ctx.AddColumns(r, cols, []zng.Value{zng.NewTime(ts), zng.NewString("S2")})
	require.NoError(t, err)
	assert.EqualValues(t, 123456000000, r.Ts())
	assert.EqualValues(t, zng.NewString("S1"), r.Value(0))
	assert.EqualValues(t, zng.NewTime(ts), r.Value(1))
	assert.EqualValues(t, zng.NewString("S2"), r.Value(2))
	zv, _ := r.Slice(4)
	assert.Nil(t, zv)
}

func TestDuplicates(t *testing.T) {
	ctx := resolver.NewContext()
	setType := ctx.LookupTypeSet(zng.TypeInt32)
	typ1, err := ctx.LookupTypeRecord([]zng.Column{
		zng.NewColumn("a", zng.TypeString),
		zng.NewColumn("b", setType),
	})
	require.NoError(t, err)
	typ2, err := ctx.LookupByName("record[a:string,b:set[int32]]")
	require.NoError(t, err)
	assert.EqualValues(t, typ1.ID(), typ2.ID())
	assert.EqualValues(t, setType.ID(), typ2.(*zng.TypeRecord).Columns[1].Type.ID())
	typ3, err := ctx.LookupByName("set[int32]")
	require.NoError(t, err)
	assert.Equal(t, setType.ID(), typ3.ID())
}

func TestTranslateAlias(t *testing.T) {
	c1 := resolver.NewContext()
	c2 := resolver.NewContext()
	set1, err := c1.LookupByName("set[int64]")
	require.NoError(t, err)
	set2, err := c2.LookupByName("set[int64]")
	require.NoError(t, err)
	alias1, err := c1.LookupTypeAlias("foo", set1)
	require.NoError(t, err)
	alias2, err := c2.LookupTypeAlias("foo", set2)
	require.NoError(t, err)
	alias3, err := c2.TranslateType(alias1)
	require.NoError(t, err)
	assert.Equal(t, alias2, alias3)
}

func TestCopyMutateColumns(t *testing.T) {
	c := resolver.NewContext()
	cols := []zng.Column{{"foo", zng.TypeString}, {"bar", zng.TypeInt64}}
	typ, err := c.LookupTypeRecord(cols)
	require.NoError(t, err)
	cols[0].Type = nil
	require.NotNil(t, typ.Columns[0].Type)
}
