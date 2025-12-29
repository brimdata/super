package parser

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/brimdata/super/compiler/ast"
)

func sliceOf[E any](s any) []E {
	if s == nil {
		return nil
	}
	slice := s.([]any)
	out := make([]E, len(slice))
	for i, el := range slice {
		out[i] = el.(E)
	}
	return out
}

func newPrimitive(c *current, typ, text string) *ast.Primitive {
	return &ast.Primitive{
		Kind: "Primitive",
		Type: typ,
		Text: text,
		Loc:  loc(c),
	}
}

func makeBinaryExprChain(first, rest any, c *current) any {
	ret := first.(ast.Expr)
	for _, p := range rest.([]any) {
		part := p.([]any)
		ret = &ast.BinaryExpr{
			Kind: "BinaryExpr",
			Op:   part[0].(string),
			LHS:  ret,
			RHS:  part[1].(ast.Expr),
			Loc:  loc(c),
		}
	}
	return ret
}

func makeArgMap(args any) (any, error) {
	m := make(map[string]any)
	for _, a := range args.([]any) {
		arg := a.(map[string]any)
		name := arg["name"].(string)
		if _, ok := m[name]; ok {
			return nil, fmt.Errorf("Duplicate argument -%s", name)
		}
		m[name] = arg["value"]
	}
	return m, nil
}

func newCall(c *current, name, args any) ast.Expr {
	return &ast.CallExpr{
		Kind: "CallExpr",
		Func: name.(ast.Expr),
		Args: sliceOf[ast.Expr](args),
		Loc:  loc(c),
	}
}

func loc(c *current) ast.Loc {
	return ast.NewLoc(c.pos.offset, c.pos.offset+len(c.text)-1)
}

func prepend(first, rest any) []any {
	return append([]any{first}, rest.([]any)...)
}

func joinChars(in any) string {
	str := bytes.Buffer{}
	for _, i := range in.([]any) {
		// handle joining bytes or strings
		if s, ok := i.([]byte); ok {
			str.Write(s)
		} else {
			str.WriteString(i.(string))
		}
	}
	return str.String()
}

func parseInt(v any) any {
	num := v.(string)
	i, err := strconv.Atoi(num)
	if err != nil {
		return nil
	}
	return i
}

func makeUnicodeChar(chars any) string {
	var r rune
	for _, char := range chars.([]any) {
		if char != nil {
			var v byte
			ch := char.([]byte)[0]
			switch {
			case ch >= '0' && ch <= '9':
				v = ch - '0'
			case ch >= 'a' && ch <= 'f':
				v = ch - 'a' + 10
			case ch >= 'A' && ch <= 'F':
				v = ch - 'A' + 10
			}
			r = (16 * r) + rune(v)
		}
	}

	return string(r)
}
