package agg

import (
	"fmt"

	"github.com/brimdata/zed"
)

type fuse struct {
	shapes   map[*zed.TypeRecord]int
	partials []zed.Value
}

var _ Function = (*fuse)(nil)

func newFuse() *fuse {
	return &fuse{
		shapes: make(map[*zed.TypeRecord]int),
	}
}

func (f *fuse) Consume(val *zed.Value) {
	// only works for record types, e.g., fuse(foo.x) where foo.x is a record
	if typ := zed.TypeRecordOf(val.Type); typ != nil {
		if _, ok := f.shapes[typ]; !ok {
			f.shapes[typ] = len(f.shapes)
		}
	}
}

func (f *fuse) Result(zctx *zed.Context) *zed.Value {
	if len(f.shapes)+len(f.partials) == 0 {
		// empty input, return type(null)... XXX singleton
		return zed.NewValue(zed.TypeType, nil)
	}
	schema := NewSchema(zctx)
	for _, p := range f.partials {
		typ, err := zctx.LookupByValue(p.Bytes)
		if err != nil {
			panic(fmt.Errorf("fuse: invalid partial value: %w", err))
		}
		recType, ok := typ.(*zed.TypeRecord)
		if !ok {
			panic(fmt.Errorf("fuse: unexpected partial type %s", typ))
		}
		schema.Mixin(recType)
	}
	shapes := make([]*zed.TypeRecord, len(f.shapes))
	for typ, i := range f.shapes {
		shapes[i] = typ
	}
	for _, typ := range shapes {
		schema.Mixin(typ)
	}
	return zctx.LookupTypeValue(schema.Type())
}

func (f *fuse) ConsumeAsPartial(partial *zed.Value) {
	if partial.Type != zed.TypeType {
		panic("fuse: partial not a type value")
	}
	f.partials = append(f.partials, *partial)
}

func (f *fuse) ResultAsPartial(zctx *zed.Context) *zed.Value {
	return f.Result(zctx)
}
