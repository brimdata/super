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
	KindNet     = 7
	KindType    = 8
	KindError   = 9
	KindArray   = 10
	KindSet     = 11
	KindMap     = 12
	KindRecord  = 13
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
	case *Array:
		return KindArray
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
	case *Net:
		return KindNet
	case *TypeValue:
		return KindType
	case *Map:
		return KindMap
	case *Record:
		return KindRecord
	case *Set:
		return KindSet
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
	case "Array":
		return KindArray
	case "Set":
		return KindSet
	case "Map":
		return KindMap
	case "Record":
		return KindRecord
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
	case *super.TypeOfNet:
		return KindNet
	case *super.TypeOfType:
		return KindType
	case *super.TypeArray:
		return KindArray
	case *super.TypeSet:
		return KindSet
	case *super.TypeMap:
		return KindMap
	case *super.TypeRecord:
		return KindRecord
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

func CompareOpToString(op int) string {
	switch op {
	case CompLT:
		return "<"
	case CompLE:
		return "<="
	case CompGT:
		return ">"
	case CompGE:
		return ">="
	case CompEQ:
		return "=="
	case CompNE:
		return "!="
	}
	panic("CompareOpToString")
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

func ArithOpToString(op int) string {
	switch op {
	case ArithAdd:
		return "+"
	case ArithSub:
		return "-"
	case ArithMul:
		return "*"
	case ArithDiv:
		return "/"
	case ArithMod:
		return "%"
	}
	panic(op)
}

func FuncCode(op int, kind Kind, lform, rform Form) int {
	// op:4, kind:3, left:2, right:2
	return int(lform) | int(rform)<<2 | int(kind)<<4 | op<<8
}
