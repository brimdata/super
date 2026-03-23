package jsonvec

import "github.com/brimdata/super/vector"

var _ Value = (*Null)(nil)

type Null struct {
	len uint32
}

func (n *Null) OnNull() Value {
	n.len++
	return n
}

func (n *Null) OnBool(v bool) Value     { return ToUnion(n).OnBool(v) }
func (n *Null) OnString(v string) Value { return ToUnion(n).OnString(v) }
func (n *Null) OnInt(v int64) Value     { return ToUnion(n).OnInt(v) }
func (n *Null) OnFloat(v float64) Value { return ToUnion(n).OnFloat(v) }
func (n *Null) BeginRecord() Value      { return ToUnion(n).BeginRecord() }
func (n *Null) Field(v string) Value    { panic("system error") }
func (n *Null) EndRecord()              { panic("system error") }
func (n *Null) BeginArray() Value       { return ToUnion(n).BeginArray() }
func (n *Null) EnterArray() Value       { panic("system error") }
func (n *Null) EndArray(Value)          { panic("system error") }
func (n *Null) Kind() vector.Kind       { return vector.KindNull }
func (n *Null) Len() uint32             { return n.len }
