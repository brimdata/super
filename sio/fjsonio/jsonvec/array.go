package jsonvec

import "github.com/brimdata/super/vector"

var _ Value = (*Array)(nil)

type Array struct {
	Offsets []uint32
	Inner   Value
}

func NewArray() *Array {
	return &Array{
		Offsets: []uint32{0},
		Inner:   Unknown{},
	}
}

func (a *Array) BeginArray() Value { return a }
func (a *Array) EnterArray() Value { return a.Inner }

func (a *Array) EndArray(inner Value) {
	a.Inner = inner
	n := a.Inner.Len()
	if n == 0 {
		a.Inner = new(Empty)
	}
	a.Offsets = append(a.Offsets, n)
}

func (a *Array) OnNull() Value           { return ToUnion(a).OnNull() }
func (a *Array) OnBool(v bool) Value     { return ToUnion(a).OnBool(v) }
func (a *Array) OnInt(v int64) Value     { return ToUnion(a).OnInt(v) }
func (a *Array) OnFloat(v float64) Value { return ToUnion(a).OnFloat(v) }
func (a *Array) OnString(v string) Value { return ToUnion(a).OnString(v) }
func (a *Array) BeginRecord() Value      { return ToUnion(a).BeginRecord() }
func (a *Array) Field(v string) Value    { panic("system error") }
func (a *Array) EndRecord()              { panic("system error") }
func (a *Array) Kind() vector.Kind       { return vector.KindArray }
func (a *Array) Len() uint32             { return uint32(len(a.Offsets)) - 1 }

type Empty struct {
	Unknown
}
