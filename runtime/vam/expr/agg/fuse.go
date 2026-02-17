package agg

import (
	"fmt"

	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/vector"
)

type fuse struct {
	types    map[super.Type]struct{}
	partials []super.Value
}

func newFuse() *fuse {
	return &fuse{
		types: make(map[super.Type]struct{}),
	}
}

func (f *fuse) Consume(vec vector.Any) {
	if _, ok := f.types[vec.Type()]; !ok {
		f.types[vec.Type()] = struct{}{}
	}
}

func (f *fuse) Result(sctx *super.Context) super.Value {
	if len(f.types)+len(f.partials) == 0 {
		return super.Null
	}
	fuser := samagg.NewFuserWithMissingFieldsAsNullable(sctx)
	for _, p := range f.partials {
		typ, err := sctx.LookupByValue(p.Bytes())
		if err != nil {
			panic(fmt.Errorf("fuse: invalid partial value: %w", err))
		}
		fuser.Fuse(typ)
	}
	for typ := range f.types {
		fuser.Fuse(typ)
	}
	return sctx.LookupTypeValue(fuser.Type())
}

func (f *fuse) ConsumeAsPartial(partial vector.Any) {
	if partial.Type() != super.TypeType {
		panic("fuse: partial not a type value")
	}
	for i := range partial.Len() {
		b := vector.TypeValueValue(partial, i)
		f.partials = append(f.partials, super.NewValue(super.TypeType, b))
	}
}

func (f *fuse) ResultAsPartial(sctx *super.Context) super.Value {
	return f.Result(sctx)
}
