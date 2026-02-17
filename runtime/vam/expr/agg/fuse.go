package agg

import (
	"fmt"

	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/vector"
)

type fuse struct {
	// XXX This convulated data structure of seen/types is here to preserve the
	// order of the types encountered since some tests rely upon this
	// and fusion gives different field order for records based on the
	// input order.  This can happen anyway due to spilling so it's not a
	// complete solution.  We should decide what to do here.  Maybe the
	// non-deterministic output is ok.
	seen     map[super.Type]struct{}
	types    []super.Type
	partials []super.Value
}

func newFuse() *fuse {
	return &fuse{
		seen: make(map[super.Type]struct{}),
	}
}

func (f *fuse) Consume(vec vector.Any) {
	typ := vec.Type()
	if _, ok := f.seen[typ]; !ok {
		f.seen[typ] = struct{}{}
	}
	f.types = append(f.types, typ)
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
	for _, typ := range f.types {
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
