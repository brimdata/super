package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type HasError struct{}

func (h HasError) Call(args []super.Value) super.Value {
	return super.NewBool(h.hasError(args[0].Type(), args[0].Bytes()))
}

func (h HasError) hasError(t super.Type, b scode.Bytes) bool {
	switch typ := super.TypeUnder(t).(type) {
	case *super.TypeRecord:
		it := b.Iter()
		return slices.ContainsFunc(typ.Fields, func(f super.Field) bool {
			return h.hasError(f.Type, it.Next())
		})
	case *super.TypeArray, *super.TypeSet:
		inner := super.InnerType(typ)
		for it := b.Iter(); !it.Done(); {
			if h.hasError(inner, it.Next()) {
				return true
			}
		}
		return false
	case *super.TypeMap:
		for it := b.Iter(); !it.Done(); {
			if h.hasError(typ.KeyType, it.Next()) || h.hasError(typ.ValType, it.Next()) {
				return true
			}
		}
		return false
	case *super.TypeUnion:
		return h.hasError(typ.Untag(b))
	case *super.TypeError:
		return true
	default:
		return false
	}
}
