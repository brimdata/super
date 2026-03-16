package jsonio

import (
	"github.com/RoaringBitmap/roaring/v2"
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Type uint8

const (
	Null Type = iota
	Bool
	Int
	Float
	String
	Object
	Array
)

type Value struct {
	Parent *Value
	// tags []TypeTag // one per row: Null, Bool, Int64, Float64, String, Object, Array
	tags []Type

	// typed storage — only populated slots corresponding to their tag
	Bools   bitvec.Bits
	Ints    *vector.Int
	Floats  *vector.Float
	Strings *vector.String
	Nulls   uint32

	Object *Record      // non-nil if any row was an object
	Array  *ArrayColumn // non-nil if any row was an array
}

func NewColumn(parent *Value) *Value {
	return &Value{
		Parent:  parent,
		Ints:    vector.NewIntEmpty(super.TypeInt64, 0),
		Floats:  vector.NewFloatEmpty(super.TypeFloat64, 0),
		Strings: vector.NewStringEmpty(0),
	}
}

type Record struct {
	len      uint32
	lut      map[string]int
	fields   []*Value
	presence []*roaring.Bitmap
	builder  scode.Builder
}

func NewObjectColumn() *Record {
	return &Record{lut: make(map[string]int)}
}

func (o *Record) Lookup(parent *Value, name string) *Value {
	i, ok := o.lut[name]
	if !ok {
		i = len(o.fields)
		o.lut[name] = i
		o.fields = append(o.fields, NewColumn(parent))
		o.presence = append(o.presence, roaring.New())
	}
	// scode.AppendCountedUvarint()

	o.presence[i].Add(o.len)
	return o.fields[i]
}

func (o *Record) end() {
	o.len++
}

type ArrayColumn struct {
	offsets []uint32
	element *Value
}

func NewArrayColumn(parent *Value) *ArrayColumn {
	return &ArrayColumn{
		offsets: []uint32{0},
		element: NewColumn(parent),
	}
}
