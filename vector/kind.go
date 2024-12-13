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
	KindIP      = 6
	KindType    = 7
	KindError   = 8
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
	case *Error:
		return KindError
	case *IP:
		return KindIP
	case *TypeValue:
		return KindType
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
	case "TypeValue":
		return KindType
	default:
		return KindInvalid
	}
}

func KindOfType(typ super.Type) Kind {
	switch super.TypeUnder(typ).(type) {
	case *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		return KindInt
	case *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		return KindUint
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		return KindFloat
	case *super.TypeOfString:
		return KindString
	case *super.TypeOfBytes:
		return KindBytes
	case *super.TypeOfIP:
		return KindIP
	case *super.TypeOfType:
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
