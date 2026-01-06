package agg

import (
	"fmt"

	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/vector"
)

type fuse struct {
	shapes   map[super.Type]int
	partials []super.Value
}

func newFuse() *fuse {
	return &fuse{
		shapes: make(map[super.Type]int),
	}
}

func (f *fuse) Consume(vec vector.Any) {
	if _, ok := f.shapes[vec.Type()]; !ok {
		f.shapes[vec.Type()] = len(f.shapes)
	}
}

func (f *fuse) Result(sctx *super.Context) super.Value {
	if len(f.shapes)+len(f.partials) == 0 {
		return super.NullType
	}
	schema := samagg.NewSchema(sctx)
	for _, p := range f.partials {
		typ, err := sctx.LookupByValue(p.Bytes())
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
	return sctx.LookupTypeValue(schema.Type())
}

func (f *fuse) ConsumeAsPartial(partial vector.Any) {
	if partial.Type() != super.TypeType {
		panic("fuse: partial not a type value")
	}
	for i := range partial.Len() {
		b, null := vector.TypeValueValue(partial, i)
		if null {
			continue
		}
		f.partials = append(f.partials, super.NewValue(super.TypeType, b))
	}
}

func (f *fuse) ResultAsPartial(sctx *super.Context) super.Value {
	return f.Result(sctx)
}
