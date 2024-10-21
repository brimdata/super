package agg

import (
	"fmt"

	"github.com/brimdata/super"
)

type fuse struct {
	shapes   map[super.Type]int
	partials []super.Value
}

var _ Function = (*fuse)(nil)

func newFuse() *fuse {
	return &fuse{
		shapes: make(map[super.Type]int),
	}
}

func (f *fuse) Consume(val super.Value) {
	if _, ok := f.shapes[val.Type()]; !ok {
		f.shapes[val.Type()] = len(f.shapes)
	}
}

func (f *fuse) Result(zctx *super.Context) super.Value {
	if len(f.shapes)+len(f.partials) == 0 {
		return super.NullType
	}
	schema := NewSchema(zctx)
	for _, p := range f.partials {
		typ, err := zctx.LookupByValue(p.Bytes())
		if err != nil {
			panic(fmt.Errorf("fuse: invalid partial value: %w", err))
		}
		schema.Mixin(typ)
	}
	shapes := make([]super.Type, len(f.shapes))
	for typ, i := range f.shapes {
		shapes[i] = typ
	}
	for _, typ := range shapes {
		schema.Mixin(typ)
	}
	return zctx.LookupTypeValue(schema.Type())
}

func (f *fuse) ConsumeAsPartial(partial super.Value) {
	if partial.Type() != super.TypeType {
		panic("fuse: partial not a type value")
	}
	f.partials = append(f.partials, partial.Copy())
}

func (f *fuse) ResultAsPartial(zctx *super.Context) super.Value {
	return f.Result(zctx)
}
