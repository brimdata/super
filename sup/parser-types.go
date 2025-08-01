package sup

import (
	"errors"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
)

func (p *Parser) parseType() (ast.Type, error) {
	typ, err := p.matchType()
	if typ == nil && err == nil {
		err = p.error("couldn't parse type")
	}
	return typ, err
}

func (p *Parser) matchType() (ast.Type, error) {
	typ, err := p.matchTypeComponent()
	if err != nil {
		return nil, err
	}
	if ok, _ := p.lexer.match('|'); ok {
		return p.matchTypeUnion(typ)
	}
	return typ, nil
}

func (p *Parser) matchTypeComponent() (ast.Type, error) {
	if typ, err := p.matchTypeName(); typ != nil || err != nil {
		return typ, err
	}
	if typ, err := p.matchTypeRecord(); typ != nil || err != nil {
		return typ, err
	}
	if typ, err := p.matchTypeArray(); typ != nil || err != nil {
		return typ, err
	}
	if typ, err := p.matchTypeSetOrMap(); typ != nil || err != nil {
		return typ, err
	}
	if typ, err := p.matchTypeParens(); typ != nil || err != nil {
		return typ, err
	}
	// no match
	return nil, nil
}

func (p *Parser) matchIdentifier() (string, error) {
	l := p.lexer
	if err := l.skipSpace(); err != nil {
		return "", err
	}
	r, _, err := l.peekRune()
	if err != nil || !idChar(r) {
		return "", err
	}
	return l.scanIdentifier()
}

func (p *Parser) matchTypeName() (ast.Type, error) {
	l := p.lexer
	if err := l.skipSpace(); err != nil {
		return nil, err
	}
	r, _, err := l.peekRune()
	if err != nil {
		return nil, err
	}
	if !(idChar(r) || isDigit(r) || r == '"') {
		return nil, nil
	}
	name, err := l.scanTypeName()
	if err != nil {
		return nil, err
	}
	if name == "error" {
		return p.matchTypeErrorBody()
	}
	if name == "enum" {
		return p.matchTypeEnumBody()
	}
	if t := super.LookupPrimitive(name); t != nil {
		return &ast.TypePrimitive{Kind: "TypePrimitive", Name: name}, nil
	}
	// Wherever we have a type name, we can have a type def defining the
	// type name.
	if ok, err := l.match('='); !ok || err != nil {
		return &ast.TypeName{Kind: "TypeName", Name: name}, nil
	}
	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}
	return &ast.TypeDef{
		Kind: "TypeDef",
		Name: name,
		Type: typ,
	}, nil
}

func (p *Parser) matchTypeRecord() (*ast.TypeRecord, error) {
	l := p.lexer
	if ok, err := l.match('{'); !ok || err != nil {
		return nil, err
	}
	var fields []ast.TypeField
	for {
		field, err := p.matchTypeField()
		if err != nil {
			return nil, err
		}
		if field == nil {
			break
		}
		fields = append(fields, *field)
		ok, err := l.match(',')
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
	}
	ok, err := l.match('}')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("mismatched braces while parsing record type")
	}
	return &ast.TypeRecord{
		Kind:   "TypeRecord",
		Fields: fields,
	}, nil
}

func (p *Parser) matchTypeField() (*ast.TypeField, error) {
	l := p.lexer
	symbol, ok, err := p.matchSymbol()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	ok, err = l.match(':')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.errorf("no type name found for field %q", symbol)
	}
	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}
	return &ast.TypeField{
		Name: symbol,
		Type: typ,
	}, nil
}

func (p *Parser) matchTypeArray() (*ast.TypeArray, error) {
	l := p.lexer
	if ok, err := l.match('['); !ok || err != nil {
		return nil, err
	}
	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}
	ok, err := l.match(']')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("mismatched brackets while parsing array type")
	}
	return &ast.TypeArray{
		Kind: "TypeArray",
		Type: typ,
	}, nil
}

func (p *Parser) matchTypeSetOrMap() (ast.Type, error) {
	l := p.lexer
	if ok, err := l.match('|'); !ok || err != nil {
		return nil, err
	}
	isSet, err := l.matchTight('[')
	if err != nil {
		return nil, err
	}
	var typ ast.Type
	var which string
	if isSet {
		which = "set"
		inner, err := p.parseType()
		if err != nil {
			return nil, err
		}
		ok, err := l.match(']')
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.error("mismatched set-brackets while parsing set type")
		}
		typ = &ast.TypeSet{
			Kind: "TypeSet",
			Type: inner,
		}
	} else {
		ok, err := l.matchTight('{')
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.error("no '|[' or '|{' type token at '|' character")
		}
		which = "map"
		typ, err = p.parseTypeMap()
		if err != nil {
			return nil, err
		}
		ok, err = l.match('}')
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.error("mismatched set-brackets while parsing map type")
		}
	}
	ok, err := l.matchTight('|')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.errorf("mismatched closing bracket while parsing type %q", which)
	}
	return typ, nil

}

func (p *Parser) parseTypeMap() (*ast.TypeMap, error) {
	keyType, err := p.parseType()
	if err != nil {
		return nil, err
	}
	ok, err := p.lexer.match(':')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("value type missing while parsing map type")
	}
	valType, err := p.parseType()
	if err != nil {
		return nil, err
	}
	return &ast.TypeMap{
		Kind:    "TypeMap",
		KeyType: keyType,
		ValType: valType,
	}, nil
}

func (p *Parser) matchTypeParens() (ast.Type, error) {
	l := p.lexer
	if ok, err := l.match('('); !ok || err != nil {
		return nil, err
	}
	typ, err := p.matchType()
	if err != nil {
		return nil, err
	}
	ok, err := l.match(')')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("mismatched parentheses while parsing parenthesized type")
	}
	return typ, nil
}

func (p *Parser) matchTypeUnion(first ast.Type) (*ast.TypeUnion, error) {
	l := p.lexer
	var types []ast.Type
	if first != nil {
		types = append(types, first)
	}
	for {
		typ, err := p.matchTypeComponent()
		if err != nil {
			return nil, err
		}
		if typ == nil {
			break
		}
		types = append(types, typ)
		ok, err := l.match('|')
		if noEOF(err) != nil {
			return nil, err
		}
		if !ok {
			break
		}
	}
	if len(types) < 2 {
		if ok, _ := l.match(','); ok {
			return nil, errors.New("union components are separated by pipe symbol (|) not comma")
		}
		return nil, errors.New("union type must include two or more types")
	}
	return &ast.TypeUnion{
		Kind:  "TypeUnion",
		Types: types,
	}, nil
}

func (p *Parser) matchTypeEnumBody() (*ast.TypeEnum, error) {
	l := p.lexer
	if ok, err := l.match('('); !ok || err != nil {
		return nil, errors.New("no opening parenthesis in enum type")
	}
	fields, err := p.matchEnumSymbols()
	if err != nil {
		return nil, err
	}
	ok, err := l.match(')')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("mismatched parentheses while parsing enum type")
	}
	return &ast.TypeEnum{
		Kind:    "TypeEnum",
		Symbols: fields,
	}, nil
}

func (p *Parser) matchEnumSymbols() ([]*ast.Text, error) {
	l := p.lexer
	var symbols []*ast.Text
	for {
		name, ok, err := p.matchSymbol()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		symbols = append(symbols, &ast.Text{Kind: "Text", Text: name})
		ok, err = l.match(',')
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
	}
	return symbols, nil
}

func (p *Parser) matchTypeErrorBody() (*ast.TypeError, error) {
	l := p.lexer
	if ok, err := l.match('('); !ok || err != nil {
		return nil, errors.New("no opening parenthesis in error type")
	}
	inner, err := p.matchType()
	if err != nil {
		return nil, err
	}
	ok, err := l.match(')')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("mismatched parentheses while parsing error type")
	}
	return &ast.TypeError{
		Kind: "TypeError",
		Type: inner,
	}, nil
}
