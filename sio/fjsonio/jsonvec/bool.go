package jsonvec

import (
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Bool struct {
	Value bitvec.Bits
}

var _ Value = (*Bool)(nil)

func (b *Bool) OnBool(v bool) Value {
	b.Value.Append(v)
	return b
}

func (b *Bool) OnNull() Value           { return ToUnion(b).OnNull() }
func (b *Bool) OnString(v string) Value { return ToUnion(b).OnString(v) }
func (b *Bool) OnInt(v int64) Value     { return ToUnion(b).OnInt(v) }
func (b *Bool) OnFloat(v float64) Value { return ToUnion(b).OnFloat(v) }
func (b *Bool) BeginRecord() Value      { return ToUnion(b).BeginRecord() }
func (b *Bool) Field(k string) Value    { panic("system error") }
func (b *Bool) EndRecord()              { panic("system error") }
func (b *Bool) BeginArray() Value       { return ToUnion(b).BeginArray() }
func (b *Bool) EnterArray() Value       { panic("system error") }
func (b *Bool) EndArray(Value)          { panic("system error") }
func (b *Bool) Kind() vector.Kind       { return vector.KindBool }
func (b *Bool) Len() uint32             { return b.Value.Len() }
