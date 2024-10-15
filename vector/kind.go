package vector

import (
	"fmt"

	"github.com/brimdata/super"
)

type Kind int
type Form int

const (
	KindInvalid = 0
	KindInt     = 1
	KindUint    = 2
	KindFloat   = 3
	KindString  = 4
	KindBytes   = 5
	KindType    = 6
)

const (
	FormFlat  = 0
	FormDict  = 1
	FormView  = 2
	FormConst = 3
)

//XXX might not need Kind...

func KindOf(v Any) Kind {
	switch v := v.(type) {
	case *Int:
		return KindInt
	case *Uint:
		return KindUint
	case *Float:
		return KindFloat
	case *Bytes:
		return KindBytes
	case *String:
		return KindString
	case *Dict:
		return KindOf(v.Any)
	case *View:
		return KindOf(v.Any)
	case *Const:
		return KindOfType(v.Value().Type())
	default:
		return KindInvalid
	}
}

func KindFromString(v string) Kind {
	switch v {
	case "Int":
		return KindInt
	case "Uint":
		return KindUint
	case "Float":
		return KindFloat
	case "Bytes":
		return KindBytes
	case "String":
		return KindString
	default:
		return KindInvalid
	}
}

func KindOfType(typ zed.Type) Kind {
	switch zed.TypeUnder(typ).(type) {
	case *zed.TypeOfInt16, *zed.TypeOfInt32, *zed.TypeOfInt64, *zed.TypeOfDuration, *zed.TypeOfTime:
		return KindInt
	case *zed.TypeOfUint16, *zed.TypeOfUint32, *zed.TypeOfUint64:
		return KindUint
	case *zed.TypeOfFloat16, *zed.TypeOfFloat32, *zed.TypeOfFloat64:
		return KindFloat
	case *zed.TypeOfString:
		return KindString
	case *zed.TypeOfBytes:
		return KindBytes
	case *zed.TypeOfType:
		return KindType
	}
	return KindInvalid
}

func FormOf(v Any) (Form, bool) {
	switch v.(type) {
	case *Int, *Uint, *Float, *Bytes, *String, *TypeValue: //XXX IP, Net
		return FormFlat, true
	case *Dict:
		return FormDict, true
	case *View:
		return FormView, true
	case *Const:
		return FormConst, true
	default:
		return 0, false
	}
}

func (f Form) String() string {
	switch f {
	case FormFlat:
		return "Flat"
	case FormDict:
		return "Dict"
	case FormView:
		return "View"
	case FormConst:
		return "Const"
	default:
		return fmt.Sprintf("Form-Unknown-%d", f)
	}
}

const (
	CompLT = 0
	CompLE = 1
	CompGT = 2
	CompGE = 3
	CompEQ = 4
	CompNE = 6
)

func CompareOpFromString(op string) int {
	switch op {
	case "<":
		return CompLT
	case "<=":
		return CompLE
	case ">":
		return CompGT
	case ">=":
		return CompGE
	case "==":
		return CompEQ
	case "!=":
		return CompNE
	}
	panic("CompareOpFromString")
}

const (
	ArithAdd = iota
	ArithSub
	ArithMul
	ArithDiv
	ArithMod
)

func ArithOpFromString(op string) int {
	switch op {
	case "+":
		return ArithAdd
	case "-":
		return ArithSub
	case "*":
		return ArithMul
	case "/":
		return ArithDiv
	case "%":
		return ArithMod
	}
	panic(op)
}

func FuncCode(op int, kind Kind, lform, rform Form) int {
	// op:3, kind:3, left:2, right:2
	return int(lform) | int(rform)<<2 | int(kind)<<4 | op<<7
}
