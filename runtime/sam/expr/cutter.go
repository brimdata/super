package expr

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
)

type Cutter struct {
	sctx       *super.Context
	fieldRefs  field.List
	fieldExprs []Evaluator
	lvals      []*Lval
	outTypes   *super.TypeVectorTable
	typeCache  []super.Type

	builders     map[string]*recordBuilderCachedTypes
	droppers     map[string]*Dropper
	dropperCache []*Dropper
	dirty        bool
}

// NewCutter returns a Cutter for fieldnames. If complement is true,
// the Cutter copies fields that are not in fieldnames. If complement
// is false, the Cutter copies any fields in fieldnames, where targets
// specifies the copied field names.
func NewCutter(sctx *super.Context, fieldRefs []*Lval, fieldExprs []Evaluator) *Cutter {
	n := len(fieldRefs)
	return &Cutter{
		sctx:         sctx,
		builders:     make(map[string]*recordBuilderCachedTypes),
		fieldRefs:    make(field.List, n),
		fieldExprs:   fieldExprs,
		lvals:        fieldRefs,
		outTypes:     super.NewTypeVectorTable(),
		typeCache:    make([]super.Type, n),
		droppers:     make(map[string]*Dropper),
		dropperCache: make([]*Dropper, n),
	}
}

func (c *Cutter) FoundCut() bool {
	return c.dirty
}

// Apply returns a new record comprising fields copied from in according to the
// receiver's configuration.  If the resulting record would be empty, Apply
// returns super.Missing.
func (c *Cutter) Eval(ectx Context, in super.Value) super.Value {
	rb, paths, err := c.lookupBuilder(ectx, in)
	if err != nil {
		return c.sctx.WrapError(fmt.Sprintf("cut: %s", err), in)
	}
	types := c.typeCache
	rb.Reset()
	droppers := c.dropperCache[:0]
	for k, e := range c.fieldExprs {
		val := e.Eval(ectx, in)
		if val.IsQuiet() {
			// ignore this field
			pathID := paths[k].String()
			if c.droppers[pathID] == nil {
				c.droppers[pathID] = NewDropper(c.sctx, field.List{paths[k]})
			}
			droppers = append(droppers, c.droppers[pathID])
			rb.Append(val.Bytes())
			types[k] = super.TypeNull
			continue
		}
		rb.Append(val.Bytes())
		types[k] = val.Type()
	}
	bytes, err := rb.Encode()
	if err != nil {
		panic(err)
	}
	rec := super.NewValue(rb.Type(c.outTypes.Lookup(types), types), bytes)
	for _, d := range droppers {
		rec = d.Eval(ectx, rec)
	}
	if !rec.IsError() {
		c.dirty = true
	}
	return rec
}

func (c *Cutter) lookupBuilder(ectx Context, in super.Value) (*recordBuilderCachedTypes, field.List, error) {
	paths := c.fieldRefs[:0]
	for _, p := range c.lvals {
		path, err := p.Eval(ectx, in)
		if err != nil {
			return nil, nil, err
		}
		if path.IsEmpty() {
			return nil, nil, errors.New("'this' not allowed (use record literal)")
		}
		paths = append(paths, path)
	}
	builder, ok := c.builders[paths.String()]
	if !ok {
		var err error
		if builder, err = newRecordBuilderCachedTypes(c.sctx, paths); err != nil {
			return nil, nil, err
		}
		c.builders[paths.String()] = builder
	}
	return builder, paths, nil
}

type recordBuilderCachedTypes struct {
	*super.RecordBuilder
	recordTypes map[int]*super.TypeRecord
}

func newRecordBuilderCachedTypes(sctx *super.Context, paths field.List) (*recordBuilderCachedTypes, error) {
	b, err := super.NewRecordBuilder(sctx, paths)
	if err != nil {
		return nil, err
	}
	return &recordBuilderCachedTypes{
		RecordBuilder: b,
		recordTypes:   make(map[int]*super.TypeRecord),
	}, nil
}

func (r *recordBuilderCachedTypes) Type(id int, types []super.Type) *super.TypeRecord {
	typ, ok := r.recordTypes[id]
	if !ok {
		typ = r.RecordBuilder.Type(types)
		r.recordTypes[id] = typ
	}
	return typ
}
