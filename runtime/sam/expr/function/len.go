package function

import (
	"github.com/brimdata/super"
)

func TypeLength(typ super.Type) int {
	switch typ := typ.(type) {
	case *super.TypeNamed:
		return TypeLength(typ.Type)
	case *super.TypeRecord:
		return len(typ.Fields)
	case *super.TypeUnion:
		return len(typ.Types)
	case *super.TypeSet:
		return TypeLength(typ.Type)
	case *super.TypeArray:
		return TypeLength(typ.Type)
	case *super.TypeEnum:
		return len(typ.Symbols)
	case *super.TypeMap:
		return TypeLength(typ.ValType)
	case *super.TypeError:
		return TypeLength(typ.Type)
	default:
		// Primitive type
		return 1
	}
}
