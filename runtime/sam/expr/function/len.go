package function

import (
	"unicode/utf8"

	"github.com/brimdata/super"
)

type LenFn struct {
	sctx *super.Context
}

func (l *LenFn) Call(args []super.Value) super.Value {
	val := args[0].Under()
	var length int
	switch typ := super.TypeUnder(val.Type()).(type) {
	case *super.TypeOfNull:
	case *super.TypeRecord:
		length = len(typ.Fields)
	case *super.TypeArray, *super.TypeSet, *super.TypeMap:
		var err error
		length, err = val.ContainerLength()
		if err != nil {
			panic(err)
		}
	case *super.TypeOfString:
		length = utf8.RuneCount(val.Bytes())
	case *super.TypeOfBytes, *super.TypeOfIP, *super.TypeOfNet:
		length = len(val.Bytes())
	case *super.TypeError:
		return l.sctx.WrapError("len()", val)
	case *super.TypeOfType:
		t, err := l.sctx.LookupByValue(val.Bytes())
		if err != nil {
			return l.sctx.NewError(err)
		}
		length = TypeLength(t)
	default:
		return l.sctx.WrapError("len: bad type", val)
	}
	return super.NewInt64(int64(length))
}

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
