//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"strings"

	"github.com/brimdata/super/vector"
)

var opToAlpha = map[string]string{
	"+": "Add",
	"-": "Sub",
	"*": "Mul",
	"/": "Div",
	"%": "Mod",
}

func main() {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "// Code generated by genarithfuncs.go. DO NOT EDIT.")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "package expr")
	fmt.Fprintln(&buf, "import (")
	fmt.Fprintln(&buf, `"github.com/brimdata/super"`)
	fmt.Fprintln(&buf, `"github.com/brimdata/super/vector"`)
	fmt.Fprintln(&buf, `"github.com/brimdata/super/vector/bitvec"`)
	fmt.Fprintln(&buf, ")")

	var ents strings.Builder
	for _, op := range []string{"+", "-", "*", "/", "%"} {
		for _, typ := range []string{"Int", "Uint", "Float", "String"} {
			if typ == "Float" && op == "%" ||
				typ == "String" && op != "+" {
				continue
			}
			for lform := vector.Form(0); lform < 4; lform++ {
				for rform := vector.Form(0); rform < 4; rform++ {
					name := "arith" + opToAlpha[op] + typ + lform.String() + rform.String()
					fmt.Fprintln(&buf, genFunc(name, op, typ, lform, rform))
					funcCode := vector.FuncCode(vector.ArithOpFromString(op), vector.KindFromString(typ), lform, rform)
					fmt.Fprintf(&ents, "%d: %s,\n", funcCode, name)
				}
			}
		}
	}

	fmt.Fprintln(&buf, "var arithFuncs = map[int]func(vector.Any, vector.Any) vector.Any{")
	fmt.Fprintln(&buf, ents.String())
	fmt.Fprintln(&buf, "}")

	out, formatErr := format.Source(buf.Bytes())
	if formatErr != nil {
		// Write unformatted source so we can find the error.
		out = buf.Bytes()
	}
	const fileName = "arithfuncs.go"
	if err := os.WriteFile(fileName, out, 0644); err != nil {
		log.Fatal(err)
	}
	if formatErr != nil {
		log.Fatal(fileName, ":", formatErr)
	}
}

func genFunc(name, op, typ string, lhs, rhs vector.Form) string {
	s := fmt.Sprintf("func %s(lhs, rhs vector.Any) vector.Any {\n", name)
	s += genVarInit("l", typ, lhs)
	s += genVarInit("r", typ, rhs)
	if lhs == vector.FormConst && rhs == vector.FormConst {
		if typ == "String" {
			s += fmt.Sprintf("val := super.NewString(lconst %s rconst)\n", op)
		} else {
			s += fmt.Sprintf("val := super.New%s(lhs.Type(), lconst %s rconst)\n", typ, op)
		}
		s += "return vector.NewConst(val, lhs.Len(), bitvec.Zero)\n"
	} else {
		s += "n := lhs.Len()\n"
		if typ == "String" {
			s += "out := vector.NewStringEmpty(n, bitvec.Zero)\n"
		} else {
			s += fmt.Sprintf("out := vector.New%sEmpty(lhs.Type(), n, bitvec.Zero)\n", typ)
		}
		s += genLoop(op, typ, lhs, rhs)
		s += "return out\n"
	}
	s += "}\n"
	return s
}

func genVarInit(which, typ string, form vector.Form) string {
	switch form {
	case vector.FormFlat:
		return fmt.Sprintf("%s := %shs.(*vector.%s)\n", which, which, typ)
	case vector.FormDict, vector.FormView:
		s := fmt.Sprintf("%sd := %shs.(*vector.%s)\n", which, which, form)
		s += fmt.Sprintf("%s := %sd.Any.(*vector.%s)\n", which, which, typ)
		s += fmt.Sprintf("%sx := %sd.Index\n", which, which)
		return s
	case vector.FormConst:
		s := fmt.Sprintf("%s := %shs.(*vector.Const)\n", which, which)
		s += fmt.Sprintf("%sconst, _ := %s.As%s()\n", which, which, typ)
		return s
	default:
		panic("genVarInit: bad form")
	}
}

func genLoop(op, typ string, lform, rform vector.Form) string {
	lexpr := genExpr("l", lform)
	rexpr := genExpr("r", rform)
	if typ == "Bytes" {
		lexpr = "string(" + lexpr + ")"
		rexpr = "string(" + rexpr + ")"
	}
	return fmt.Sprintf("for k := uint32(0); k < n; k++ { out.Append(%s %s %s) }\n", lexpr, op, rexpr)
}

func genExpr(which string, form vector.Form) string {
	switch form {
	case vector.FormFlat:
		return which + ".Value(k)"
	case vector.FormDict, vector.FormView:
		return fmt.Sprintf("%s.Value(uint32(%sx[k]))", which, which)
	case vector.FormConst:
		return which + "const"
	}
	panic(form)
}
