package jsonvec

import (
	"encoding/json"

	"github.com/brimdata/super/pkg/field"
	"github.com/bytedance/sonic/ast"
)

type projBuilder struct {
	Builder
	builder Builder
	nop     nopBuilder
	stack   []*trie
}

func NewProjectionBuilder(proj field.List) Builder {
	b := NewBuilder()
	return &projBuilder{
		Builder: b,
		builder: b,
		stack:   []*trie{newTrie(proj)},
	}
}

func (p *projBuilder) OnObjectBegin(capacity int) error {
	p.push(nil)
	return p.Builder.OnObjectBegin(capacity)
}

func (p *projBuilder) OnObjectKey(name string) error {
	p.pop()
	t := p.tos()
	var next *trie
	if t.leaf {
		next = t
		p.Builder = p.builder
	} else if c, ok := t.get(name); ok {
		next = c
		p.Builder = p.builder
	} else {
		p.Builder = p.nop
	}
	p.push(next)
	return p.Builder.OnObjectKey(name)
}

func (p *projBuilder) OnObjectEnd() error {
	p.pop()
	if p.tos() != nil {
		p.Builder = p.builder
	}
	return p.Builder.OnObjectEnd()
}

func (p *projBuilder) Value() Value {
	return p.builder.Value()
}

func (b *projBuilder) tos() *trie {
	return b.stack[len(b.stack)-1]
}

func (b *projBuilder) push(t *trie) {
	b.stack = append(b.stack, t)
}

func (b *projBuilder) pop() *trie {
	v := b.tos()
	b.stack = b.stack[:len(b.stack)-1]
	return v
}

type trie struct {
	children map[string]*trie
	leaf     bool
}

func newTrie(paths field.List) *trie {
	root := &trie{}
	for _, p := range paths {
		n := root
		for _, s := range p {
			if n.children == nil {
				n.children = map[string]*trie{}
			}
			c, ok := n.children[s]
			if !ok {
				c = &trie{}
				n.children[s] = c
			}
			n = c
		}
		n.leaf = true
	}
	return root
}

func (t *trie) get(field string) (*trie, bool) {
	if t == nil {
		return nil, false
	}
	trie, ok := t.children[field]
	return trie, ok
}

type nopBuilder struct{}

func (nopBuilder) OnNull() error                            { return nil }
func (nopBuilder) OnBool(v bool) error                      { return nil }
func (nopBuilder) OnString(v string) error                  { return nil }
func (nopBuilder) OnInt64(v int64, n json.Number) error     { return nil }
func (nopBuilder) OnFloat64(v float64, n json.Number) error { return nil }
func (nopBuilder) OnObjectBegin(capacity int) error         { return ast.VisitOPSkip }
func (nopBuilder) OnObjectKey(key string) error             { return nil }
func (nopBuilder) OnObjectEnd() error                       { return nil }
func (nopBuilder) OnArrayBegin(capacity int) error          { return ast.VisitOPSkip }
func (nopBuilder) OnArrayEnd() error                        { return nil }
func (nopBuilder) Value() Value                             { return new(Empty) }
