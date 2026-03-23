package jsonvec

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

var _ Value = (*Int)(nil)

type Int struct {
	Value *vector.Int
}

func NewInt() *Int {
	return &Int{Value: vector.NewIntEmpty(super.TypeInt64, 0)}
}

func (i *Int) OnInt(v int64) Value {
	i.Value.Append(v)
	return i
}

func (i *Int) OnNull() Value           { return ToUnion(i).OnNull() }
func (i *Int) OnBool(v bool) Value     { return ToUnion(i).OnBool(v) }
func (i *Int) OnFloat(v float64) Value { return ToUnion(i).OnFloat(v) }
func (i *Int) OnString(v string) Value { return ToUnion(i).OnString(v) }
func (i *Int) BeginRecord() Value      { return ToUnion(i).BeginRecord() }
func (i *Int) Field(key string) Value  { panic("system error") }
func (i *Int) EndRecord()              { panic("system error") }
func (i *Int) BeginArray() Value       { return ToUnion(i).BeginArray() }
func (i *Int) EnterArray() Value       { panic("system error") }
func (i *Int) EndArray(Value)          { panic("system error") }
func (i *Int) Kind() vector.Kind       { return vector.KindInt }
func (i *Int) Len() uint32             { return i.Value.Len() }
