package jsonvec

import (
	"github.com/brimdata/super/vector"
)

type Value interface {
	OnNull() Value
	OnBool(bool) Value
	OnInt(int64) Value
	OnFloat(float64) Value
	OnString(string) Value
	BeginRecord() Value
	Field(string) Value
	EndRecord()
	BeginArray() Value
	EnterArray() Value
	EndArray(Value)
	Kind() vector.Kind
	Len() uint32
}

var _ Value = Unknown{}

type Unknown struct{}

func (Unknown) OnNull() Value           { return new(Null).OnNull() }
func (Unknown) OnBool(v bool) Value     { return new(Bool).OnBool(v) }
func (Unknown) OnInt(v int64) Value     { return NewInt().OnInt(v) }
func (Unknown) OnFloat(v float64) Value { return NewFloat().OnFloat(v) }
func (Unknown) OnString(v string) Value { return NewString().OnString(v) }
func (Unknown) BeginRecord() Value      { return NewRecord().BeginRecord() }
func (Unknown) Field(string) Value      { panic("system error") }
func (Unknown) EndRecord()              { panic("system error") }
func (Unknown) BeginArray() Value       { return NewArray() }
func (Unknown) EnterArray() Value       { panic("system error") }
func (Unknown) EndArray(Value)          { panic("system error") }
func (Unknown) Kind() vector.Kind       { panic("system error") }
func (Unknown) Len() uint32             { return 0 }
