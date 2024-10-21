package zson_test

import (
	"bytes"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zngio"
	"github.com/brimdata/super/zson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Thing interface {
	Color() string
}

type Plant struct {
	MyColor string
}

func (p *Plant) Color() string { return p.MyColor }

type Animal struct {
	MyColor string
}

func (a *Animal) Color() string { return a.MyColor }

func TestInterfaceMarshal(t *testing.T) {
	m := zson.NewMarshaler()
	m.Decorate(zson.StyleSimple)

	zsonRose, err := m.Marshal(Thing(&Plant{"red"}))
	require.NoError(t, err)
	assert.Equal(t, `{MyColor:"red"}(=Plant)`, zsonRose)

	zsonFlamingo, err := m.Marshal(Thing(&Animal{"pink"}))
	require.NoError(t, err)
	assert.Equal(t, `{MyColor:"pink"}(=Animal)`, zsonFlamingo)

	u := zson.NewUnmarshaler()
	u.Bind(Plant{}, Animal{})
	var thing Thing

	err = u.Unmarshal(zsonRose, &thing)
	require.NoError(t, err)
	assert.Equal(t, "red", thing.Color())

	err = u.Unmarshal(zsonFlamingo, &thing)
	require.NoError(t, err)
	assert.Equal(t, "pink", thing.Color())
}

type Roll bool

func TestMarshal(t *testing.T) {
	z, err := zson.Marshal("hello, world")
	require.NoError(t, err)
	assert.Equal(t, `"hello, world"`, z)

	aIn := []int8{1, 2, 3}
	z, err = zson.Marshal(aIn)
	require.NoError(t, err)
	assert.Equal(t, `[1(int8),2(int8),3(int8)]`, z)

	var v interface{}
	err = zson.Unmarshal(z, &v)
	require.NoError(t, err)
	aOut, ok := v.([]int8)
	assert.Equal(t, ok, true)
	assert.Equal(t, aIn, aOut)

	m := zson.NewMarshaler()
	m.Decorate(zson.StyleSimple)
	z, err = m.Marshal(Roll(true))
	require.NoError(t, err)
	assert.Equal(t, `true(=Roll)`, z)
}

type BytesRecord struct {
	B []byte
}

type BytesArrayRecord struct {
	A [3]byte
}

type ID [4]byte

type IDRecord struct {
	A ID
	B ID
}

type IDSlice []byte

type SliceRecord struct {
	S []IDSlice
}

func TestBytes(t *testing.T) {
	m := zson.NewZNGMarshaler()
	rec, err := m.Marshal(BytesRecord{B: []byte{1, 2, 3}})
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{B:0x010203}", zson.FormatValue(rec))

	rec, err = m.Marshal(BytesArrayRecord{A: [3]byte{4, 5, 6}})
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{A:0x040506}", zson.FormatValue(rec))

	id := IDRecord{A: ID{0, 1, 2, 3}, B: ID{4, 5, 6, 7}}
	m = zson.NewZNGMarshaler()
	m.Decorate(zson.StyleSimple)
	rec, err = m.Marshal(id)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{A:0x00010203(=ID),B:0x04050607(ID)}(=IDRecord)", zson.FormatValue(rec))

	var id2 IDRecord
	u := zson.NewZNGUnmarshaler()
	u.Bind(IDRecord{}, ID{})
	err = zson.UnmarshalZNG(rec, &id2)
	require.NoError(t, err)
	assert.Equal(t, id, id2)

	b2 := BytesRecord{B: nil}
	m = zson.NewZNGMarshaler()
	rec, err = m.Marshal(b2)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{B:null(bytes)}", zson.FormatValue(rec))

	s := SliceRecord{S: nil}
	m = zson.NewZNGMarshaler()
	rec, err = m.Marshal(s)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{S:null([bytes])}", zson.FormatValue(rec))
}

type RecordWithInterfaceSlice struct {
	X string
	S []Thing
}

func TestMixedTypeArrayInsideRecord(t *testing.T) {
	t.Skip("see issue #4012")
	x := &RecordWithInterfaceSlice{
		X: "hello",
		S: []Thing{
			&Plant{"red"},
			&Animal{"blue"},
		},
	}
	m := zson.NewZNGMarshaler()
	m.Decorate(zson.StyleSimple)

	zv, err := m.Marshal(x)
	require.NoError(t, err)

	var buffer bytes.Buffer
	writer := zngio.NewWriter(zio.NopCloser(&buffer))
	recExpected := super.NewValue(zv.Type(), zv.Bytes())
	writer.Write(recExpected)
	writer.Close()

	reader := zngio.NewReader(super.NewContext(), &buffer)
	defer reader.Close()
	recActual, err := reader.Read()
	exp := zson.FormatValue(recExpected)
	actual := zson.FormatValue(*recActual)
	assert.Equal(t, exp, actual)
	// Double check that all the proper typing made it into the implied union.
	assert.Equal(t, `{X:"hello",S:[[{MyColor:"red"}(=Plant),{MyColor:"blue"}(=Animal)]]}(=RecordWithInterfaceSlice)`, actual)

	u := zson.NewUnmarshaler()
	u.Bind(Animal{}, Plant{}, RecordWithInterfaceSlice{})
	var out RecordWithInterfaceSlice
	err = u.Unmarshal(actual, &out)
	require.NoError(t, err)
	assert.Equal(t, *x, out)
}

type ArrayOfThings struct {
	S []Thing
}

func TestMixedTypeUnmarshal(t *testing.T) {
	u := zson.NewUnmarshaler()
	u.Bind(Animal{}, Plant{}, ArrayOfThings{})
	var out ArrayOfThings
	err := u.Unmarshal(`{S:[{MyColor:"red"}(=Plant),{MyColor:"blue"}(=Animal)]}`, &out)
	require.NoError(t, err)
	assert.Equal(t, ArrayOfThings{S: []Thing{&Plant{"red"}, &Animal{"blue"}}}, out)
}

type MessageThing struct {
	Message string
	Thing   Thing
}

func TestMixedTypeArrayOfStructWithInterface(t *testing.T) {
	t.Skip("see issue #4012")
	input := []MessageThing{
		{
			Message: "hello",
			Thing:   &Plant{"red"},
		},
		{
			Message: "world",
			Thing:   &Animal{"blue"},
		},
	}
	m := zson.NewZNGMarshaler()
	m.Decorate(zson.StyleSimple)

	zv, err := m.Marshal(input)
	require.NoError(t, err)

	var buffer bytes.Buffer
	writer := zngio.NewWriter(zio.NopCloser(&buffer))
	recExpected := super.NewValue(zv.Type(), zv.Bytes())
	writer.Write(recExpected)
	writer.Close()

	reader := zngio.NewReader(super.NewContext(), &buffer)
	defer reader.Close()
	recActual, err := reader.Read()
	require.NoError(t, err)
	exp := zson.FormatValue(recExpected)
	actual := zson.FormatValue(*recActual)
	assert.Equal(t, exp, actual)
	// Double check that all the proper typing made it into the implied union.
	assert.Equal(t, `[{Message:"hello",Thing:{MyColor:"red"}(=Plant)}(=MessageThing),{Message:"world",Thing:{MyColor:"blue"}(=Animal)}(=MessageThing)]`, actual)

	u := zson.NewUnmarshaler()
	u.Bind(Plant{}, Animal{}, MessageThing{})
	var out RecordWithInterfaceSlice
	err = u.Unmarshal(actual, &out)
	require.NoError(t, err)
	assert.Equal(t, input, out)
}

type Foo struct {
	A int
	a int
}

func TestUnexported(t *testing.T) {
	f := &Foo{1, 2}
	m := zson.NewZNGMarshaler()
	_, err := m.Marshal(f)
	require.NoError(t, err)
}

type ZNGValueField struct {
	Name  string
	Field super.Value `zed:"field"`
}

func TestZNGValueField(t *testing.T) {
	// Include a Zed int64 inside a Go struct as a super.Value field.
	zngValueField := &ZNGValueField{
		Name:  "test1",
		Field: super.NewInt64(123),
	}
	m := zson.NewZNGMarshaler()
	m.Decorate(zson.StyleSimple)
	zv, err := m.Marshal(zngValueField)
	require.NoError(t, err)
	assert.Equal(t, `{Name:"test1",field:123}(=ZNGValueField)`, zson.FormatValue(zv))
	u := zson.NewZNGUnmarshaler()
	var out ZNGValueField
	err = u.Unmarshal(zv, &out)
	require.NoError(t, err)
	assert.Equal(t, zngValueField.Name, out.Name)
	assert.True(t, zngValueField.Field.Equal(out.Field))
	// Include a Zed record inside a Go struct in a super.Value field.
	zv2, err := zson.ParseValue(super.NewContext(), `{s:"foo",a:[1,2,3]}`)
	require.NoError(t, err)
	zngValueField2 := &ZNGValueField{
		Name:  "test2",
		Field: zv2,
	}
	m2 := zson.NewZNGMarshaler()
	m2.Decorate(zson.StyleSimple)
	zv3, err := m2.Marshal(zngValueField2)
	require.NoError(t, err)
	assert.Equal(t, `{Name:"test2",field:{s:"foo",a:[1,2,3]}}(=ZNGValueField)`, zson.FormatValue(zv3))
	u2 := zson.NewZNGUnmarshaler()
	var out2 ZNGValueField
	err = u2.Unmarshal(zv3, &out2)
	require.NoError(t, err)
	assert.Equal(t, *zngValueField2, out2)
}

func TestJSONFieldTag(t *testing.T) {
	type jsonTag struct {
		Value string `json:"value"`
	}
	s, err := zson.Marshal(jsonTag{Value: "test"})
	require.NoError(t, err)
	assert.Equal(t, `{value:"test"}`, s)
	var j jsonTag
	require.NoError(t, zson.Unmarshal(s, &j))
	assert.Equal(t, jsonTag{Value: "test"}, j)
}

func TestIgnoreField(t *testing.T) {
	type s struct {
		Value  string       `zed:"value"`
		Ignore func() error `zed:"-"`
	}
	b, err := zson.Marshal(s{Value: "test"})
	require.NoError(t, err)
	assert.Equal(t, `{value:"test"}`, b)
	var v s
	require.NoError(t, zson.Unmarshal(b, &v))
	assert.Equal(t, s{Value: "test"}, v)
}

func TestMarshalNetIP(t *testing.T) {
	before := net.ParseIP("10.0.0.1")
	b, err := zson.Marshal(before)
	require.NoError(t, err)
	assert.Equal(t, `10.0.0.1`, b)
	var after net.IP
	err = zson.Unmarshal(b, &after)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestMarshalNetipAddr(t *testing.T) {
	before := netip.MustParseAddr("10.0.0.1")
	b, err := zson.Marshal(before)
	require.NoError(t, err)
	assert.Equal(t, `10.0.0.1`, b)
	var after netip.Addr
	err = zson.Unmarshal(b, &after)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestMarshalDecoratedIPs(t *testing.T) {
	m := zson.NewMarshaler()
	// Make sure IPs don't get decorated with Go type and just
	// appear as native Zed IPs.
	m.Decorate(zson.StyleSimple)
	b, err := m.Marshal(net.ParseIP("142.250.72.142"))
	require.NoError(t, err)
	assert.Equal(t, `142.250.72.142`, b)
	b, err = m.Marshal(netip.MustParseAddr("142.250.72.142"))
	require.NoError(t, err)
	assert.Equal(t, `142.250.72.142`, b)
}

func TestMarshalGoTime(t *testing.T) {
	tm, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05.123Z")
	b, err := zson.Marshal(tm)
	require.NoError(t, err)
	assert.Equal(t, `2006-01-02T15:04:05.123Z`, b)
}

type Metadata interface {
	Type() super.Type
}

type Record struct {
	Fields []Field
}

func (r *Record) Type() super.Type {
	return super.TypeNull
}

type Field struct {
	Name   string
	Values Metadata
}

type Primitive struct {
	Foo string
}

func (*Primitive) Type() super.Type {
	return super.TypeNull
}

type Array struct {
	Values Metadata
}

func (*Array) Type() super.Type {
	return super.TypeNull
}

func TestRecordWithMixedTypeNamedArrayElems(t *testing.T) {
	in := &Record{
		Fields: []Field{
			{
				Name: "a",
				Values: &Primitive{
					Foo: "foo",
				},
			},
			{
				Name: "b",
				Values: &Array{
					Values: &Primitive{
						Foo: "bar",
					},
				},
			},
		},
	}
	m := zson.NewZNGMarshaler()
	m.Decorate(zson.StyleSimple)
	val, err := m.Marshal(in)
	require.NoError(t, err)
	u := zson.NewZNGUnmarshaler()
	u.Bind(Record{}, Array{}, Primitive{})
	var out Metadata
	err = u.Unmarshal(val, &out)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestInterfaceWithConcreteEmptyValue(t *testing.T) {
	u := zson.NewUnmarshaler()
	// This case doesn't need a binding because we set the
	// interface value to an empty underlying value.
	out := Metadata(&Primitive{})
	err := u.Unmarshal(`{Foo:"foo"}(=Primitive)`, &out)
	require.NoError(t, err)
	assert.Equal(t, &Primitive{Foo: "foo"}, out)
}

func TestZedType(t *testing.T) {
	zctx := super.NewContext()
	u := zson.NewUnmarshaler()
	var typ super.Type
	err := u.Unmarshal(`<string>`, &typ)
	assert.EqualError(t, err, `cannot unmarshal type value without type context`)
	u.SetContext(zctx)
	err = u.Unmarshal(`<string>`, &typ)
	require.NoError(t, err)
	assert.Equal(t, super.TypeString, typ)
	err = u.Unmarshal(`<int64>`, &typ)
	require.NoError(t, err)
	assert.Equal(t, super.TypeInt64, typ)
}

func TestSimpleUnionUnmarshal(t *testing.T) {
	t.Skip("see issue #4012")
	var i int64
	err := zson.Unmarshal(`1((int64,string))`, &i)
	require.NoError(t, err)
	assert.Equal(t, 1, i)
}

func TestEmbeddedNilInterface(t *testing.T) {
	in := &Record{
		Fields: nil,
	}
	val, err := zson.Marshal(in)
	require.NoError(t, err)
	assert.Equal(t, `{Fields:null([{Name:string,Values:null}])}`, val)
}
