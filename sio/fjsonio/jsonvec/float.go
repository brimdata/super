package jsonvec

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

var _ Value = (*Float)(nil)

type Float struct {
	Value *vector.Float
}

func NewFloat() *Float {
	return &Float{Value: vector.NewFloatEmpty(super.TypeFloat64, 0)}
}

func (f *Float) OnFloat(v float64) Value {
	f.Value.Append(v)
	return f
}

func (f *Float) OnNull() Value           { return ToUnion(f).OnNull() }
func (f *Float) OnBool(v bool) Value     { return ToUnion(f).OnBool(v) }
func (f *Float) OnInt(v int64) Value     { return ToUnion(f).OnInt(v) }
func (f *Float) OnString(v string) Value { return ToUnion(f).OnString(v) }
func (f *Float) BeginRecord() Value      { return ToUnion(f).BeginRecord() }
func (f *Float) Field(key string) Value  { panic("system error") }
func (f *Float) EndRecord()              { panic("system error") }
func (f *Float) BeginArray() Value       { return ToUnion(f).BeginArray() }
func (f *Float) EnterArray() Value       { panic("system error") }
func (f *Float) EndArray(Value)          { panic("system error") }
func (f *Float) Kind() vector.Kind       { return vector.KindFloat }
func (f *Float) Len() uint32             { return f.Value.Len() }
