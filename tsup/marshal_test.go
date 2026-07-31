package tsup_test

import (
	"bytes"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/tsup"
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
	m := tsup.NewMarshaler()
	m.Decorate(tsup.StyleSimple)

	supRose, err := m.Marshal(Thing(&Plant{"red"}))
	require.NoError(t, err)
	assert.Equal(t, `type Plant={MyColor:string}
{MyColor:"red"}::Plant`, supRose)

	supFlamingo, err := m.Marshal(Thing(&Animal{"pink"}))
	require.NoError(t, err)
	assert.Equal(t, `type Animal={MyColor:string}
{MyColor:"pink"}::Animal`, supFlamingo)

	u := tsup.NewUnmarshaler()
	u.Bind(Plant{}, Animal{})
	var thing Thing

	err = u.Unmarshal(supRose, &thing)
	require.NoError(t, err)
	assert.Equal(t, "red", thing.Color())

	err = u.Unmarshal(supFlamingo, &thing)
	require.NoError(t, err)
	assert.Equal(t, "pink", thing.Color())
}

type Roll bool

func TestMarshal(t *testing.T) {
	z, err := tsup.Marshal("hello, world")
	require.NoError(t, err)
	assert.Equal(t, `"hello, world"`, z)

	aIn := []int8{1, 2, 3}
	z, err = tsup.Marshal(aIn)
	require.NoError(t, err)
	assert.Equal(t, `[1::int8,2::int8,3::int8]`, z)

	var v any
	err = tsup.Unmarshal(z, &v)
	require.NoError(t, err)
	aOut, ok := v.([]int8)
	assert.Equal(t, ok, true)
	assert.Equal(t, aIn, aOut)

	m := tsup.NewMarshaler()
	m.Decorate(tsup.StyleSimple)
	z, err = m.Marshal(Roll(true))
	require.NoError(t, err)
	assert.Equal(t, "type Roll=bool\ntrue::Roll", z)
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
	m := tsup.NewBSUPMarshaler()
	rec, err := m.Marshal(BytesRecord{B: []byte{1, 2, 3}})
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{B:0x010203}", tsup.FormatValue(rec))

	rec, err = m.Marshal(BytesArrayRecord{A: [3]byte{4, 5, 6}})
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{A:0x040506}", tsup.FormatValue(rec))

	id := IDRecord{A: ID{0, 1, 2, 3}, B: ID{4, 5, 6, 7}}
	m = tsup.NewBSUPMarshaler()
	m.Decorate(tsup.StyleSimple)
	rec, err = m.Marshal(id)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, `type ID=bytes
type IDRecord={A:ID,B:ID}
{A:0x00010203,B:0x04050607}::IDRecord`, tsup.FormatValueWithTypes(rec))

	var id2 IDRecord
	u := tsup.NewBSUPUnmarshaler()
	u.Bind(IDRecord{}, ID{})
	err = tsup.UnmarshalBSUP(rec, &id2)
	require.NoError(t, err)
	assert.Equal(t, id, id2)

	b2 := BytesRecord{B: nil}
	m = tsup.NewBSUPMarshaler()
	rec, err = m.Marshal(b2)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{B:0x}", tsup.FormatValue(rec))

	s := SliceRecord{S: nil}
	m = tsup.NewBSUPMarshaler()
	rec, err = m.Marshal(s)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "{S:[]::[bytes]}", tsup.FormatValue(rec))
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
	m := tsup.NewBSUPMarshaler()
	m.Decorate(tsup.StyleSimple)

	zv, err := m.Marshal(x)
	require.NoError(t, err)

	var buffer bytes.Buffer
	writer := bsupio.NewWriter(sio.NopCloser(&buffer))
	recExpected := super.NewValue(zv.Type(), zv.Bytes())
	writer.Write(recExpected)
	writer.Close()

	reader := bsupio.NewReader(super.NewContext(), &buffer)
	defer reader.Close()
	recActual, err := reader.Read()
	exp := tsup.FormatValue(recExpected)
	actual := tsup.FormatValue(*recActual)
	assert.Equal(t, exp, actual)
	// Double check that all the proper typing made it into the implied union.
	assert.Equal(t, `{X:"hello",S:[[{MyColor:"red"}::=Plant,{MyColor:"blue"}::=Animal]]}:=RecordWithInterfaceSlice`, actual)

	u := tsup.NewUnmarshaler()
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
	const in = `
		type Animal={MyColor:string}
		type Plant={MyColor:string}
		{S:[{MyColor:"red"}::Plant,{MyColor:"blue"}::Animal]}
		`
	u := tsup.NewUnmarshaler()
	u.Bind(Animal{}, Plant{}, ArrayOfThings{})
	var out ArrayOfThings
	err := u.Unmarshal(in, &out)
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
	m := tsup.NewBSUPMarshaler()
	m.Decorate(tsup.StyleSimple)

	zv, err := m.Marshal(input)
	require.NoError(t, err)

	var buffer bytes.Buffer
	writer := bsupio.NewWriter(sio.NopCloser(&buffer))
	recExpected := super.NewValue(zv.Type(), zv.Bytes())
	writer.Write(recExpected)
	writer.Close()

	reader := bsupio.NewReader(super.NewContext(), &buffer)
	defer reader.Close()
	recActual, err := reader.Read()
	require.NoError(t, err)
	exp := tsup.FormatValue(recExpected)
	actual := tsup.FormatValue(*recActual)
	assert.Equal(t, exp, actual)
	// Double check that all the proper typing made it into the implied union.
	assert.Equal(t, `[{Message:"hello",Thing:{MyColor:"red"}::=Plant}::=MessageThing,{Message:"world",Thing:{MyColor:"blue"}::=Animal}::=MessageThing]`, actual)

	u := tsup.NewUnmarshaler()
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
	m := tsup.NewBSUPMarshaler()
	_, err := m.Marshal(f)
	require.NoError(t, err)
}

type BSUPValueField struct {
	Name  string
	Field super.Value `super:"field"`
}

func TestBSUPValueField(t *testing.T) {
	// Include an int64 inside a Go struct as a super.Value field.
	bsupValueField := &BSUPValueField{
		Name:  "test1",
		Field: super.NewInt64(123),
	}
	m := tsup.NewBSUPMarshaler()
	m.Decorate(tsup.StyleSimple)
	zv, err := m.Marshal(bsupValueField)
	require.NoError(t, err)
	assert.Equal(t, `type BSUPValueField={Name:string,field:any}
{Name:"test1",field:123::any}::BSUPValueField`, tsup.FormatValueWithTypes(zv))
	u := tsup.NewBSUPUnmarshaler()
	var out BSUPValueField
	err = u.Unmarshal(zv, &out)
	require.NoError(t, err)
	assert.Equal(t, bsupValueField.Name, out.Name)
	assert.True(t, bsupValueField.Field.Equal(out.Field))
	// Include a record inside a Go struct in a super.Value field.
	zv2, err := tsup.ParseValue(super.NewContext(), `{s:"foo",a:[1,2,3]}`)
	require.NoError(t, err)
	bsupValueField2 := &BSUPValueField{
		Name:  "test2",
		Field: zv2,
	}
	m2 := tsup.NewBSUPMarshaler()
	m2.Decorate(tsup.StyleSimple)
	zv3, err := m2.Marshal(bsupValueField2)
	require.NoError(t, err)
	assert.Equal(t, `type BSUPValueField={Name:string,field:any}
{Name:"test2",field:{s:"foo",a:[1,2,3]}::any}::BSUPValueField`, tsup.FormatValueWithTypes(zv3))
	u2 := tsup.NewBSUPUnmarshaler()
	var out2 BSUPValueField
	err = u2.Unmarshal(zv3, &out2)
	require.NoError(t, err)
	assert.Equal(t, *bsupValueField2, out2)
}

func TestJSONFieldTag(t *testing.T) {
	type jsonTag struct {
		Value string `json:"value"`
	}
	s, err := tsup.Marshal(jsonTag{Value: "test"})
	require.NoError(t, err)
	assert.Equal(t, `{value:"test"}`, s)
	var j jsonTag
	require.NoError(t, tsup.Unmarshal(s, &j))
	assert.Equal(t, jsonTag{Value: "test"}, j)
}

func TestIgnoreField(t *testing.T) {
	type s struct {
		Value  string       `super:"value"`
		Ignore func() error `super:"-"`
	}
	b, err := tsup.Marshal(s{Value: "test"})
	require.NoError(t, err)
	assert.Equal(t, `{value:"test"}`, b)
	var v s
	require.NoError(t, tsup.Unmarshal(b, &v))
	assert.Equal(t, s{Value: "test"}, v)
}

func TestMarshalNetIP(t *testing.T) {
	before := net.ParseIP("10.0.0.1")
	b, err := tsup.Marshal(before)
	require.NoError(t, err)
	assert.Equal(t, `10.0.0.1`, b)
	var after net.IP
	err = tsup.Unmarshal(b, &after)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestMarshalNetipAddr(t *testing.T) {
	before := netip.MustParseAddr("10.0.0.1")
	b, err := tsup.Marshal(before)
	require.NoError(t, err)
	assert.Equal(t, `10.0.0.1`, b)
	var after netip.Addr
	err = tsup.Unmarshal(b, &after)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestMarshalDecoratedIPs(t *testing.T) {
	m := tsup.NewMarshaler()
	// Make sure IPs don't get decorated with Go type and just
	// appear as native super-structured IPs.
	m.Decorate(tsup.StyleSimple)
	b, err := m.Marshal(net.ParseIP("142.250.72.142"))
	require.NoError(t, err)
	assert.Equal(t, `142.250.72.142`, b)
	b, err = m.Marshal(netip.MustParseAddr("142.250.72.142"))
	require.NoError(t, err)
	assert.Equal(t, `142.250.72.142`, b)
}

func TestMarshalGoTime(t *testing.T) {
	tm, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05.123Z")
	b, err := tsup.Marshal(tm)
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
	t.Skip() // skipping until we fix marshal to use named types for interfaces
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
	m := tsup.NewBSUPMarshaler()
	m.Decorate(tsup.StyleSimple)
	val, err := m.Marshal(in)
	require.NoError(t, err)
	u := tsup.NewBSUPUnmarshaler()
	u.Bind(Record{}, Array{}, Primitive{})
	var out Metadata
	err = u.Unmarshal(val, &out)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestInterfaceWithConcreteEmptyValue(t *testing.T) {
	u := tsup.NewUnmarshaler()
	// This case doesn't need a binding because we set the
	// interface value to an empty underlying value.
	out := Metadata(&Primitive{})
	err := u.Unmarshal(`type Primitive={Foo:string} {Foo:"foo"}::Primitive`, &out)
	require.NoError(t, err)
	assert.Equal(t, &Primitive{Foo: "foo"}, out)
}

func TestSuperType(t *testing.T) {
	sctx := super.NewContext()
	u := tsup.NewUnmarshaler()
	var typ super.Type
	err := u.Unmarshal(`<string>`, &typ)
	assert.EqualError(t, err, `cannot unmarshal type value without type context`)
	u.SetContext(sctx)
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
	err := tsup.Unmarshal(`1::int64|string`, &i)
	require.NoError(t, err)
	assert.Equal(t, 1, i)
}

func TestEmbeddedNilInterface(t *testing.T) {
	in := &Record{
		Fields: nil,
	}
	val, err := tsup.Marshal(in)
	require.NoError(t, err)
	assert.Equal(t, `{Fields:[]::[{Name:string,Values:null}]}`, val)
}
