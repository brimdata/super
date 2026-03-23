package jsonvec

import (
	"encoding/binary"

	"github.com/brimdata/super/vector"
)

var _ Value = (*Record)(nil)

type Record struct {
	Fields []*Element
	RLEs   []vector.RLE
	LUT    map[string]int

	tags []uint32
	// typeToTag map[super.Type]uint32
	typeToTag map[string]uint32
	scratch   []byte
}

func NewRecord() *Record {
	return &Record{
		LUT:       make(map[string]int),
		typeToTag: make(map[string]uint32),
	}
}

func (r *Record) BeginRecord() Value {
	r.scratch = r.scratch[:0]
	return r
}

func (r *Record) Field(name string) Value {
	idx, ok := r.LUT[name]
	if !ok {
		idx = len(r.Fields)
		r.LUT[name] = idx
		r.Fields = append(r.Fields, &Element{Value: Unknown{}})
		r.RLEs = append(r.RLEs, vector.RLE{})
	}
	r.scratch = binary.AppendUvarint(r.scratch, uint64(idx))
	f := r.Fields[idx]
	r.RLEs[idx].Touch(uint32(len(r.tags)))
	return f
}

func (r *Record) EndRecord() {
	tag, ok := r.typeToTag[string(r.scratch)]
	if !ok {
		tag = uint32(len(r.typeToTag))
		r.typeToTag[string(r.scratch)] = tag
	}
	r.tags = append(r.tags, tag)
}

func (r *Record) OnNull() Value           { return ToUnion(r).OnNull() }
func (r *Record) OnBool(v bool) Value     { return ToUnion(r).OnBool(v) }
func (r *Record) OnInt(v int64) Value     { return ToUnion(r).OnInt(v) }
func (r *Record) OnFloat(v float64) Value { return ToUnion(r).OnFloat(v) }
func (r *Record) OnString(v string) Value { return ToUnion(r).OnString(v) }
func (r *Record) BeginArray() Value       { return ToUnion(r).BeginArray() }
func (r *Record) EnterArray() Value       { panic("system error") }
func (r *Record) EndArray(Value)          {}
func (r *Record) Kind() vector.Kind       { return vector.KindRecord }
func (r *Record) Len() uint32             { return uint32(len(r.tags)) }

var _ Value = (*Element)(nil)

type Element struct {
	Value
}

func (f *Element) OnNull() Value {
	f.Value = f.Value.OnNull()
	return f
}

func (f *Element) OnBool(v bool) Value {
	f.Value = f.Value.OnBool(v)
	return f
}

func (f *Element) OnString(v string) Value {
	f.Value = f.Value.OnString(v)
	return f
}

func (f *Element) OnInt(v int64) Value {
	f.Value = f.Value.OnInt(v)
	return f
}

func (f *Element) OnFloat(v float64) Value {
	f.Value = f.Value.OnFloat(v)
	return f
}

func (f *Element) BeginRecord() Value {
	f.Value = f.Value.BeginRecord()
	return f
}

func (f *Element) BeginArray() Value {
	f.Value = f.Value.BeginArray()
	return f
}
