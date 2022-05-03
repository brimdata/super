package index

import (
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/pkg/field"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
)

type Rule interface {
	CreateTime() nano.Ts
	RuleName() string
	RuleID() ksuid.KSUID
	RuleKeys() field.List
	Zed() string
	String() string
}

type FieldRule struct {
	Ts     nano.Ts     `zed:"ts"`
	ID     ksuid.KSUID `zed:"id"`
	Name   string      `zed:"name"`
	Fields field.List  `zed:"fields,omitempty"`
}

type TypeRule struct {
	Ts   nano.Ts     `zed:"ts"`
	ID   ksuid.KSUID `zed:"id"`
	Name string      `zed:"name"`
	Type string      `zed:"type"`
}

type AggRule struct {
	Ts     nano.Ts     `zed:"ts"`
	ID     ksuid.KSUID `zed:"id"`
	Name   string      `zed:"name"`
	Script string      `zed:"script"`
}

func NewFieldRule(name, keys string) *FieldRule {
	fields := field.DottedList(keys)
	if len(fields) != 1 {
		//XXX fix this
		panic("NewFieldRule: only one key supported")
	}
	return &FieldRule{
		Ts:     nano.Now(),
		Name:   name,
		ID:     ksuid.New(),
		Fields: fields,
	}
}

func NewTypeRule(name string, typ zed.Type) *TypeRule {
	return &TypeRule{
		Ts:   nano.Now(),
		Name: name,
		ID:   ksuid.New(),
		Type: zson.FormatType(typ),
	}
}

func NewAggRule(name, prog string) (*AggRule, error) {
	// make sure it compiles
	if _, err := compiler.ParseOp(prog); err != nil {
		return nil, err
	}
	return &AggRule{
		Ts:     nano.Now(),
		Name:   name,
		ID:     ksuid.New(),
		Script: prog,
	}, nil
}

// Equivalent returns true if the two rules create the same index object.
func Equivalent(a, b Rule) bool {
	switch ra := a.(type) {
	case *FieldRule:
		if rb, ok := b.(*FieldRule); ok {
			return ra.Fields.Equal(rb.Fields)
		}
	case *TypeRule:
		if rb, ok := b.(*TypeRule); ok {
			return ra.Type == rb.Type
		}
	case *AggRule:
		if rb, ok := b.(*AggRule); ok {
			return ra.Script == rb.Script
		}
	}
	return false
}

const fieldZed = `
cut %s
| put key := this, count := count() - 1
| count := count(), seek.min := min(count), seek.max := max(count) by key
| sort %s`

func (f *FieldRule) Zed() string {
	return fmt.Sprintf(fieldZed, f.Fields, f.RuleKeys())
}

func (t *TypeRule) Zed() string {
	// XXX See issue #3140 as this does not allow for multiple type keys
	return fmt.Sprintf("explode this by %s as key | count() by key | sort key", t.Type)
}

func (a *AggRule) Zed() string {
	return a.Script
}

func (f *FieldRule) String() string {
	return fmt.Sprintf("rule %s field %s", f.ID, f.Fields)
}

func (t *TypeRule) String() string {
	return fmt.Sprintf("rule %s type %s", t.ID, t.Type)
}

func (a *AggRule) String() string {
	return fmt.Sprintf("rule %s agg %q", a.ID, a.Script)
}

func (f *FieldRule) CreateTime() nano.Ts {
	return f.Ts
}

func (t *TypeRule) CreateTime() nano.Ts {
	return t.Ts
}

func (a *AggRule) CreateTime() nano.Ts {
	return a.Ts
}

func (f *FieldRule) RuleName() string {
	return f.Name
}

func (t *TypeRule) RuleName() string {
	return t.Name
}

func (a *AggRule) RuleName() string {
	return a.Name
}

func (f *FieldRule) RuleID() ksuid.KSUID {
	return f.ID
}

func (t *TypeRule) RuleID() ksuid.KSUID {
	return t.ID
}

func (a *AggRule) RuleID() ksuid.KSUID {
	return a.ID
}

func (f *FieldRule) RuleKeys() field.List {
	keys := make(field.List, len(f.Fields))
	for i, path := range f.Fields {
		keys[i] = append(field.Path{"key"}, path...)
	}
	return keys
}

func (t *TypeRule) RuleKeys() field.List {
	return field.DottedList("key")
}

func (a *AggRule) RuleKeys() field.List {
	// XXX can get these by analyzing the compiled script
	return nil
}
