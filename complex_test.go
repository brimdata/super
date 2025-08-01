package super_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zio/supio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordAccessNamed(t *testing.T) {
	const input = `{foo:"hello"::=zfile,bar:true::=zbool}::=0`
	rec := sup.MustParseValue(super.NewContext(), input)
	s := rec.Deref("foo").AsString()
	assert.Equal(t, s, "hello")
	b := rec.Deref("bar").AsBool()
	assert.Equal(t, b, true)
}

func TestNonRecordDeref(t *testing.T) {
	const input = `
1
192.168.1.1
null
[1,2,3]
|[1,2,3]|`
	reader := supio.NewReader(super.NewContext(), strings.NewReader(input))
	for {
		val, err := reader.Read()
		if val == nil {
			break
		}
		require.NoError(t, err)
		v := val.Deref("foo")
		require.Nil(t, v)
	}
}

func TestNormalizeSet(t *testing.T) {
	t.Run("duplicate-element", func(t *testing.T) {
		b := zcode.NewBuilder()
		b.BeginContainer()
		b.Append([]byte("dup"))
		b.Append([]byte("dup"))
		b.TransformContainer(super.NormalizeSet)
		b.EndContainer()
		set := zcode.Append(nil, []byte("dup"))
		expected := zcode.Append(nil, set)
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("unsorted-elements", func(t *testing.T) {
		b := zcode.NewBuilder()
		b.BeginContainer()
		b.Append([]byte("z"))
		b.Append([]byte("a"))
		b.TransformContainer(super.NormalizeSet)
		b.EndContainer()
		set := zcode.Append(nil, []byte("a"))
		set = zcode.Append(set, []byte("z"))
		expected := zcode.Append(nil, set)
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("unsorted-and-duplicate-elements", func(t *testing.T) {
		b := zcode.NewBuilder()
		big := bytes.Repeat([]byte("x"), 256)
		small := []byte("small")
		b.Append(big)
		b.BeginContainer()
		// Append duplicate elements in reverse of set-normal order.
		for range 3 {
			b.Append(big)
			b.Append(big)
			b.Append(small)
			b.Append(small)
			b.Append(nil)
			b.Append(nil)
		}
		b.TransformContainer(super.NormalizeSet)
		b.EndContainer()
		set := zcode.Append(nil, nil)
		set = zcode.Append(set, small)
		set = zcode.Append(set, big)
		expected := zcode.Append(nil, big)
		expected = zcode.Append(expected, set)
		require.Exactly(t, expected, b.Bytes())
	})
}

func TestDuplicates(t *testing.T) {
	ctx := super.NewContext()
	setType := ctx.LookupTypeSet(super.TypeInt32)
	typ1, err := ctx.LookupTypeRecord([]super.Field{
		{"a", super.TypeString},
		{"b", setType},
	})
	require.NoError(t, err)
	typ2, err := sup.ParseType(ctx, "{a:string,b:|[int32]|}")
	require.NoError(t, err)
	assert.EqualValues(t, typ1.ID(), typ2.ID())
	assert.EqualValues(t, setType.ID(), typ2.(*super.TypeRecord).Fields[1].Type.ID())
	typ3, err := ctx.LookupByValue(super.EncodeTypeValue(setType))
	require.NoError(t, err)
	assert.Equal(t, setType.ID(), typ3.ID())
}

func TestTranslateNamed(t *testing.T) {
	c1 := super.NewContext()
	c2 := super.NewContext()
	set1, err := sup.ParseType(c1, "|[int64]|")
	require.NoError(t, err)
	set2, err := sup.ParseType(c2, "|[int64]|")
	require.NoError(t, err)
	named1, err := c1.LookupTypeNamed("foo", set1)
	require.NoError(t, err)
	named2, err := c2.LookupTypeNamed("foo", set2)
	require.NoError(t, err)
	named3, err := c2.TranslateType(named1)
	require.NoError(t, err)
	assert.Equal(t, named2, named3)
}

func TestCopyMutateFields(t *testing.T) {
	c := super.NewContext()
	fields := []super.Field{{"foo", super.TypeString}, {"bar", super.TypeInt64}}
	typ, err := c.LookupTypeRecord(fields)
	require.NoError(t, err)
	fields[0].Type = nil
	require.NotNil(t, typ.Fields[0].Type)
}
