package expr

import (
	"fmt"
	"slices"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/field"
	"github.com/brimdata/zed/zcode"
)

// Putter is an Evaluator that modifies the record stream with computed values.
// Each new value is called a clause and consists of a field name and
// an expression. Each put clause either replaces an existing value in
// the field specified or appends a value as a new field.  Appended
// values appear as new fields in the order that the clause appears
// in the put expression.
type Putter struct {
	zctx    *zed.Context
	builder zcode.Builder
	clauses []Assignment
	rules   map[int]map[string]putRule
	// vals is a slice to avoid re-allocating for every value
	vals []zed.Value
	// paths is a slice to avoid re-allocating for every path
	paths field.List
}

// A putRule describes how a given record type is modified by describing
// which input fields should be replaced with which clause expression and
// which clauses should be appended.  The type of each clause expression
// is recorded since a new rule must be created if any of the types change.
// Such changes aren't typically expected but are possible in the expression
// language.
type putRule struct {
	typ         zed.Type
	clauseTypes []zed.Type
	step        putStep
}

func NewPutter(zctx *zed.Context, clauses []Assignment) *Putter {
	return &Putter{
		zctx:    zctx,
		clauses: clauses,
		vals:    make([]zed.Value, len(clauses)),
		rules:   make(map[int]map[string]putRule),
	}
}

func (p *Putter) eval(ectx Context, this zed.Value) ([]zed.Value, field.List, error) {
	p.vals = p.vals[:0]
	p.paths = p.paths[:0]
	for _, cl := range p.clauses {
		val := cl.RHS.Eval(ectx, this)
		if val.IsQuiet() {
			continue
		}
		p.vals = append(p.vals, val)
		path, err := cl.LHS.Eval(ectx, this)
		if err != nil {
			return nil, nil, err
		}
		p.paths = append(p.paths, path)
	}
	return p.vals, p.paths, nil
}

// A putStep is a recursive data structure encoding a series of steps to be
// carried out to construct an output record from an input record and
// a slice of evaluated clauses.
type putStep struct {
	op        putOp
	index     int
	container bool
	record    []putStep // for op == record
}

func (p *putStep) append(step putStep) {
	p.record = append(p.record, step)
}

type putOp int

const (
	putFromInput  putOp = iota // copy field from input record
	putFromClause              // copy field from put assignment
	putRecord                  // recurse into record below us
)

func (p *putStep) build(in zcode.Bytes, b *zcode.Builder, vals []zed.Value) zcode.Bytes {
	switch p.op {
	case putRecord:
		b.Reset()
		if err := p.buildRecord(in, b, vals); err != nil {
			return nil
		}
		return b.Bytes()
	default:
		// top-level op must be a record
		panic(fmt.Sprintf("put: unexpected step %v", p.op))
	}
}

func (p *putStep) buildRecord(in zcode.Bytes, b *zcode.Builder, vals []zed.Value) error {
	ig := newGetter(in)

	for _, step := range p.record {
		switch step.op {
		case putFromInput:
			bytes, err := ig.nth(step.index)
			if err != nil {
				return err
			}
			b.Append(bytes)
		case putFromClause:
			b.Append(vals[step.index].Bytes())
		case putRecord:
			b.BeginContainer()
			bytes, err := in, error(nil)
			if step.index >= 0 {
				bytes, err = ig.nth(step.index)
				if err != nil {
					return err
				}
			}
			if err := step.buildRecord(bytes, b, vals); err != nil {
				return err
			}
			b.EndContainer()
		}
	}
	return nil
}

// A getter provides random access to values in a zcode container
// using zcode.Iter. It uses a cursor to avoid quadratic re-seeks for
// the common case where values are fetched sequentially.
type getter struct {
	cursor    int
	container zcode.Bytes
	it        zcode.Iter
}

func newGetter(cont zcode.Bytes) getter {
	return getter{
		cursor:    -1,
		container: cont,
		it:        cont.Iter(),
	}
}

func (ig *getter) nth(n int) (zcode.Bytes, error) {
	if n < ig.cursor {
		ig.it = ig.container.Iter()
	}
	for !ig.it.Done() {
		zv := ig.it.Next()
		ig.cursor++
		if ig.cursor == n {
			return zv, nil
		}
	}
	return nil, fmt.Errorf("getter.nth: array index %d out of bounds", n)
}

func findOverwriteClause(path field.Path, paths field.List) (int, field.Path, bool) {
	for i, lpath := range paths {
		if path.Equal(lpath) || lpath.HasStrictPrefix(path) {
			return i, lpath, true
		}
	}
	return -1, nil, false
}

func (p *Putter) deriveSteps(inType *zed.TypeRecord, vals []zed.Value, paths field.List) (putStep, zed.Type) {
	return p.deriveRecordSteps(field.Path{}, inType.Fields, vals, paths)
}

func (p *Putter) deriveRecordSteps(parentPath field.Path, inFields []zed.Field, vals []zed.Value, paths field.List) (putStep, *zed.TypeRecord) {
	s := putStep{op: putRecord}
	var fields []zed.Field

	// First look at all input fields to see which should
	// be copied over and which should be overwritten by
	// assignments.
	for i, f := range inFields {
		path := append(parentPath, f.Name)
		matchIndex, matchPath, found := findOverwriteClause(path, paths)
		switch {
		// input not overwritten by assignment: copy input value.
		case !found:
			s.append(putStep{
				op:        putFromInput,
				container: zed.IsContainerType(f.Type),
				index:     i,
			})
			fields = append(fields, f)
		// input field overwritten by non-nested assignment: copy assignment value.
		case len(path) == len(matchPath):
			s.append(putStep{
				op:        putFromClause,
				container: zed.IsContainerType(vals[matchIndex].Type()),
				index:     matchIndex,
			})
			fields = append(fields, zed.NewField(f.Name, vals[matchIndex].Type()))
		// input record field overwritten by nested assignment: recurse.
		case len(path) < len(matchPath) && zed.IsRecordType(f.Type):
			nestedStep, typ := p.deriveRecordSteps(path, zed.TypeRecordOf(f.Type).Fields, vals, paths)
			nestedStep.index = i
			s.append(nestedStep)
			fields = append(fields, zed.NewField(f.Name, typ))
		// input non-record field overwritten by nested assignment(s): recurse.
		case len(path) < len(matchPath) && !zed.IsRecordType(f.Type):
			nestedStep, typ := p.deriveRecordSteps(path, []zed.Field{}, vals, paths)
			nestedStep.index = i
			s.append(nestedStep)
			fields = append(fields, zed.NewField(f.Name, typ))
		default:
			panic("put: internal error computing record steps")
		}
	}

	appendClause := func(lpath field.Path) bool {
		if !lpath.HasPrefix(parentPath) {
			return false
		}
		return !hasField(lpath[len(parentPath)], fields)
	}
	// Then, look at put assignments to see if there are any new fields to append.
	for i, lpath := range paths {
		if appendClause(lpath) {
			switch {
			// Append value at this level
			case len(lpath) == len(parentPath)+1:
				s.append(putStep{
					op:        putFromClause,
					container: zed.IsContainerType(vals[i].Type()),
					index:     i,
				})
				fields = append(fields, zed.NewField(lpath[len(parentPath)], vals[i].Type()))
			// Appended and nest. For example, this would happen with "put b.c=1" applied to a record {"a": 1}.
			case len(lpath) > len(parentPath)+1:
				path := append(parentPath, lpath[len(parentPath)])
				nestedStep, typ := p.deriveRecordSteps(path, []zed.Field{}, vals, paths)
				nestedStep.index = -1
				fields = append(fields, zed.NewField(lpath[len(parentPath)], typ))
				s.append(nestedStep)
			}
		}
	}
	typ, err := p.zctx.LookupTypeRecord(fields)
	if err != nil {
		panic(err)
	}
	return s, typ
}

func hasField(name string, fields []zed.Field) bool {
	return slices.ContainsFunc(fields, func(f zed.Field) bool {
		return f.Name == name
	})
}

func (p *Putter) lookupRule(inType *zed.TypeRecord, vals []zed.Value, fields field.List) (putRule, error) {
	m, ok := p.rules[inType.ID()]
	if !ok {
		m = make(map[string]putRule)
		p.rules[inType.ID()] = m
	}
	rule, ok := m[fields.String()]
	if ok && sameTypes(rule.clauseTypes, vals) {
		return rule, nil
	}
	// first check fields
	if err := CheckPutFields(fields); err != nil {
		return putRule{}, fmt.Errorf("put: %w", err)
	}
	step, typ := p.deriveSteps(inType, vals, fields)
	var clauseTypes []zed.Type
	for _, val := range vals {
		clauseTypes = append(clauseTypes, val.Type())
	}
	rule = putRule{typ, clauseTypes, step}
	p.rules[inType.ID()][fields.String()] = rule
	return rule, nil
}

func CheckPutFields(fields field.List) error {
	for i, f := range fields {
		if f.IsEmpty() {
			return fmt.Errorf("left-hand side cannot be 'this' (use 'yield' operator)")
		}
		for _, c := range fields[i+1:] {
			if f.Equal(c) {
				return fmt.Errorf("multiple assignments to %s", f)
			}
			if c.HasStrictPrefix(f) {
				return fmt.Errorf("conflicting nested assignments to %s and %s", f, c)
			}
			if f.HasStrictPrefix(c) {
				return fmt.Errorf("conflicting nested assignments to %s and %s", c, f)
			}
		}
	}
	return nil
}

func sameTypes(types []zed.Type, vals []zed.Value) bool {
	return slices.EqualFunc(types, vals, func(typ zed.Type, val zed.Value) bool {
		return typ == val.Type()
	})
}

func (p *Putter) Eval(ectx Context, this zed.Value) zed.Value {
	recType := zed.TypeRecordOf(this.Type())
	if recType == nil {
		if this.IsError() {
			// propagate errors
			return this
		}
		return p.zctx.WrapError(ectx.Arena(), "put: not a record", this)
	}
	vals, paths, err := p.eval(ectx, this)
	if err != nil {
		return p.zctx.WrapError(ectx.Arena(), fmt.Sprintf("put: %s", err), this)
	}
	if len(vals) == 0 {
		return this
	}
	rule, err := p.lookupRule(recType, vals, paths)
	if err != nil {
		return p.zctx.WrapError(ectx.Arena(), err.Error(), this)
	}
	bytes := rule.step.build(this.Bytes(), &p.builder, vals)
	return ectx.Arena().New(rule.typ, bytes)
}
