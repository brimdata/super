package zfmt

import (
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/zson"
)

type canonZed struct {
	formatter
}

// XXX this needs to change when we use the zson values from the ast
func (c *canonZed) literal(e ast.Primitive) {
	switch e.Type {
	case "string", "error":
		c.write("\"%s\"", e.Text)
	case "regexp":
		c.write("/%s/", e.Text)
	default:
		//XXX need decorators for non-implied
		c.write("%s", e.Text)

	}
}

func (c *canonZed) fieldpath(path []string) {
	if len(path) == 0 {
		c.write("this")
		return
	}
	for k, s := range path {
		if zson.IsIdentifier(s) {
			if k != 0 {
				c.write(".")
			}
			c.write(s)
		} else {
			if k == 0 {
				c.write(".")
			}
			c.write("[%q]", s)
		}
	}
}

func (c *canonZed) typ(t ast.Type) {
	switch t := t.(type) {
	case *ast.TypePrimitive:
		c.write(t.Name)
	case *ast.TypeRecord:
		c.write("{")
		c.typeFields(t.Fields)
		c.write("}")
	case *ast.TypeArray:
		c.write("[")
		c.typ(t.Type)
		c.write("]")
	case *ast.TypeSet:
		c.write("|[")
		c.typ(t.Type)
		c.write("]|")
	case *ast.TypeUnion:
		c.write("(")
		c.types(t.Types)
		c.write(")")
	case *ast.TypeEnum:
		//XXX need to figure out Zed syntax for enum literal which may
		// be different than zson, requiring some ast adjustments.
		c.write("TBD:ENUM")
	case *ast.TypeMap:
		c.write("|{")
		c.typ(t.KeyType)
		c.write(":")
		c.typ(t.ValType)
		c.write("}|")
	case *ast.TypeNull:
		c.write("null")
	case *ast.TypeDef:
		c.write("%s=(", t.Name)
		c.typ(t.Type)
		c.write(")")
	case *ast.TypeName:
		c.write(t.Name)
	case *ast.TypeError:
		c.write("error(")
		c.typ(t.Type)
		c.write(")")
	}
}

func (c *canonZed) typeFields(fields []ast.TypeField) {
	for k, f := range fields {
		if k != 0 {
			c.write(",")
		}
		c.write("%s:", zson.QuotedName(f.Name))
		c.typ(f.Type)
	}
}

func (c *canonZed) types(types []ast.Type) {
	for k, t := range types {
		if k != 0 {
			c.write(",")
		}
		c.typ(t)
	}
}
