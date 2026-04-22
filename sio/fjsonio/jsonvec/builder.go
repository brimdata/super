package jsonvec

import (
	"encoding/json"

	"github.com/bytedance/sonic/ast"
)

type Builder interface {
	ast.Visitor
	Value() Value
}

var _ Builder = (*builder)(nil)

type builder struct {
	stack []Value
}

func NewBuilder() Builder {
	return &builder{
		stack: []Value{Unknown{}},
	}
}

func (b *builder) Reset() {
	b.stack = b.stack[:1]
}

func (b *builder) OnNull() error {
	b.stack[len(b.stack)-1] = b.tos().OnNull()
	return nil
}

func (b *builder) OnBool(v bool) error {
	b.stack[len(b.stack)-1] = b.tos().OnBool(v)
	return nil
}

func (b *builder) OnInt64(v int64, _ json.Number) error {
	b.stack[len(b.stack)-1] = b.tos().OnInt(v)
	return nil
}

func (b *builder) OnFloat64(v float64, _ json.Number) error {
	b.stack[len(b.stack)-1] = b.tos().OnFloat(v)
	return nil
}

func (b *builder) OnString(v string) error {
	b.stack[len(b.stack)-1] = b.tos().OnString(v)
	return nil
}

func (b *builder) OnObjectBegin(capacity int) error {
	b.stack[len(b.stack)-1] = b.tos().BeginRecord()
	b.push(nil)
	return nil
}

func (b *builder) OnObjectKey(name string) error {
	b.pop()
	b.push(b.tos().Field(name))
	return nil
}

func (b *builder) OnObjectEnd() error {
	b.pop()
	b.tos().EndRecord()
	return nil
}

func (b *builder) OnArrayBegin(capacity int) error {
	b.stack[len(b.stack)-1] = b.tos().BeginArray()
	b.push(b.tos().EnterArray())
	return nil
}

func (b *builder) OnArrayEnd() error {
	inner := b.pop()
	b.tos().EndArray(inner)
	return nil
}

func (b *builder) Value() Value {
	return b.stack[0]
}

func (b *builder) tos() Value {
	return b.stack[len(b.stack)-1]
}

func (b *builder) push(val Value) {
	b.stack = append(b.stack, val)
}

func (b *builder) pop() Value {
	v := b.tos()
	b.stack = b.stack[:len(b.stack)-1]
	return v
}
