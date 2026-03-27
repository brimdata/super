package jsonvec

import "github.com/brimdata/super/vector"

var _ Value = (*String)(nil)

type String struct {
	Value *vector.String
}

func NewString() *String {
	return &String{vector.NewStringEmpty(0)}
}

func (s *String) OnString(v string) Value {
	s.Value.Append(v)
	return s
}

func (s *String) OnNull() Value           { return ToUnion(s).OnNull() }
func (s *String) OnBool(v bool) Value     { return ToUnion(s).OnBool(v) }
func (s *String) OnInt(v int64) Value     { return ToUnion(s).OnInt(v) }
func (s *String) OnFloat(v float64) Value { return ToUnion(s).OnFloat(v) }
func (s *String) BeginRecord() Value      { return ToUnion(s).BeginRecord() }
func (s *String) Field(string) Value      { panic("system error") }
func (s *String) EndRecord()              { panic("system error") }
func (s *String) BeginArray() Value       { return ToUnion(s).BeginArray() }
func (s *String) EnterArray() Value       { panic("system error") }
func (s *String) EndArray(Value)          { panic("system error") }
func (s *String) Kind() vector.Kind       { return vector.KindString }
func (s *String) Len() uint32             { return s.Value.Len() }
