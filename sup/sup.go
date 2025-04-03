// Package sup provides fundamental interfaces to the SUP data format comprising
// Reader, Writer, Parser, and so forth.  The SUP format includes a type system
// that requries a semantic analysis to parse an input to its structured data
// representation.  To do so, Parser translats a SUP input to an AST, Analyzer
// performs semantic type analysis to turn the AST into a Value, and Builder
// constructs a super.Value from a Value.
package sup

import (
	"slices"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/zcode"
)

// Implied returns true for primitive types whose type can be inferred
// syntactically from its value and thus never needs a decorator.
func Implied(typ super.Type) bool {
	switch typ := typ.(type) {
	case *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime, *super.TypeOfFloat64, *super.TypeOfBool, *super.TypeOfBytes, *super.TypeOfString, *super.TypeOfIP, *super.TypeOfNet, *super.TypeOfType, *super.TypeOfNull:
		return true
	case *super.TypeRecord:
		return !slices.ContainsFunc(typ.Fields, func(f super.Field) bool {
			return !Implied(f.Type)
		})
	case *super.TypeArray:
		return Implied(typ.Type)
	case *super.TypeSet:
		return Implied(typ.Type)
	case *super.TypeMap:
		return Implied(typ.KeyType) && Implied(typ.ValType)
	case *super.TypeError:
		return Implied(typ.Type)
	}
	return false
}

// SelfDescribing returns true for types whose type name can be entirely derived
// from its typed value, e.g., a record type can be derived from a record value
// because all of the field names and type names are present in the value, but
// an enum type cannot be derived from an enum value because not all the enumerated
// names are present in the value.  In the former case, a decorated typedef can
// use the abbreviated form "(= <name>)", while the letter case, a type def must use
// the longer form "<value> (<name> = (<type>))".
func SelfDescribing(typ super.Type) bool {
	if Implied(typ) {
		return true
	}
	switch typ := typ.(type) {
	case *super.TypeRecord, *super.TypeArray, *super.TypeSet, *super.TypeMap:
		return true
	case *super.TypeNamed:
		return SelfDescribing(typ.Type)
	}
	return false
}

func ParseType(zctx *super.Context, sup string) (super.Type, error) {
	zp := NewParser(strings.NewReader(sup))
	ast, err := zp.parseType()
	if ast == nil || noEOF(err) != nil {
		return nil, err
	}
	return NewAnalyzer().convertType(zctx, ast)
}

func ParseValue(zctx *super.Context, sup string) (super.Value, error) {
	zp := NewParser(strings.NewReader(sup))
	ast, err := zp.ParseValue()
	if err != nil {
		return super.Null, err
	}
	val, err := NewAnalyzer().ConvertValue(zctx, ast)
	if err != nil {
		return super.Null, err
	}
	return Build(zcode.NewBuilder(), val)
}

func MustParseValue(zctx *super.Context, sup string) super.Value {
	val, err := ParseValue(zctx, sup)
	if err != nil {
		panic(err)
	}
	return val
}

func ParseValueFromAST(zctx *super.Context, ast ast.Value) (super.Value, error) {
	val, err := NewAnalyzer().ConvertValue(zctx, ast)
	if err != nil {
		return super.Null, err
	}
	return Build(zcode.NewBuilder(), val)
}

func TranslateType(zctx *super.Context, astType ast.Type) (super.Type, error) {
	return NewAnalyzer().convertType(zctx, astType)
}
