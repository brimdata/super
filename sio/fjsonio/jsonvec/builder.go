package jsonvec

import (
	"encoding/json"

	"github.com/bytedance/sonic/ast"
)

var _ ast.Visitor = (*Builder)(nil)

type Builder struct {
	stack []Value
}

func NewBuilder() *Builder {
	return &Builder{
		stack: []Value{Unknown{}},
	}
}

func (b *Builder) Reset() {
	b.stack = b.stack[:1]
}

func (b *Builder) OnNull() error {
	b.stack[len(b.stack)-1] = b.tos().OnNull()
	return nil
}

func (b *Builder) OnBool(v bool) error {
	b.stack[len(b.stack)-1] = b.tos().OnBool(v)
	return nil
}

func (b *Builder) OnInt64(v int64, _ json.Number) error {
	b.stack[len(b.stack)-1] = b.tos().OnInt(v)
	return nil
}

func (b *Builder) OnFloat64(v float64, _ json.Number) error {
	b.stack[len(b.stack)-1] = b.tos().OnFloat(v)
	return nil
}

func (b *Builder) OnString(v string) error {
	b.stack[len(b.stack)-1] = b.tos().OnString(v)
	return nil
}

func (b *Builder) OnObjectBegin(capacity int) error {
	b.stack[len(b.stack)-1] = b.tos().BeginRecord()
	b.push(nil)
	return nil
}

func (b *Builder) OnObjectKey(name string) error {
	b.pop()
	b.push(b.tos().Field(name))
	return nil
}

func (b *Builder) OnObjectEnd() error {
	b.pop()
	b.tos().EndRecord()
	return nil
}

func (b *Builder) OnArrayBegin(capacity int) error {
	b.stack[len(b.stack)-1] = b.tos().BeginArray()
	b.push(b.tos().EnterArray())
	return nil
}

func (b *Builder) OnArrayEnd() error {
	inner := b.pop()
	b.tos().EndArray(inner)
	return nil
}

func (b *Builder) tos() Value {
	return b.stack[len(b.stack)-1]
}

func (b *Builder) push(val Value) {
	b.stack = append(b.stack, val)
}

func (b *Builder) pop() Value {
	v := b.tos()
	b.stack = b.stack[:len(b.stack)-1]
	return v
}
