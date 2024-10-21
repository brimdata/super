package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#fields
type Fields struct {
	zctx *super.Context
	typ  super.Type
}

func NewFields(zctx *super.Context) *Fields {
	return &Fields{
		zctx: zctx,
		typ:  zctx.LookupTypeArray(zctx.LookupTypeArray(super.TypeString)),
	}
}

func buildPath(typ *super.TypeRecord, b *zcode.Builder, prefix []string) {
	for _, f := range typ.Fields {
		if typ, ok := super.TypeUnder(f.Type).(*super.TypeRecord); ok {
			buildPath(typ, b, append(prefix, f.Name))
		} else {
			b.BeginContainer()
			for _, s := range prefix {
				b.Append([]byte(s))
			}
			b.Append([]byte(f.Name))
			b.EndContainer()
		}
	}
}

func (f *Fields) Call(_ super.Allocator, args []super.Value) super.Value {
	subjectVal := args[0].Under()
	typ := f.recordType(subjectVal)
	if typ == nil {
		return f.zctx.Missing()
	}
	var b zcode.Builder
	buildPath(typ, &b, nil)
	return super.NewValue(f.typ, b.Bytes())
}

func (f *Fields) recordType(val super.Value) *super.TypeRecord {
	if typ, ok := super.TypeUnder(val.Type()).(*super.TypeRecord); ok {
		return typ
	}
	if val.Type() == super.TypeType {
		typ, err := f.zctx.LookupByValue(val.Bytes())
		if err != nil {
			return nil
		}
		if typ, ok := super.TypeUnder(typ).(*super.TypeRecord); ok {
			return typ
		}
	}
	return nil
}
