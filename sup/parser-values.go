package sup

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/scode"
)

func (p *Parser) ParseValue() (ast.Value, error) {
	v, err := p.matchValue()
	if err == io.EOF {
		err = nil
	}
	if v == nil && err == nil {
		if err := p.lexer.check(1); (err != nil && err != io.EOF) || len(p.lexer.cursor) > 0 {
			return nil, errors.New("SUP syntax error")
		}
	}
	return v, err
}

func noEOF(err error) error {
	if err == io.EOF {
		err = nil
	}
	return err
}

func (p *Parser) matchValue() (ast.Value, error) {
	if val, err := p.matchRecord(); val != nil || err != nil {
		return p.decorate(val, err)
	}
	if val, err := p.matchArray(); val != nil || err != nil {
		return p.decorate(val, err)
	}
	if val, err := p.matchSetOrMap(); val != nil || err != nil {
		return p.decorate(val, err)
	}
	if val, err := p.matchTypeValue(); val != nil || err != nil {
		return p.decorate(val, err)
	}
	// Primitive comes last as the other matchers short-circuit more
	// efficiently on sentinel characters.
	if val, err := p.matchPrimitive(); val != nil || err != nil {
		return p.decorate(val, err)
	}
	if val, err := p.matchError(); val != nil || err != nil {
		return p.decorate(val, err)
	}
	return nil, nil
}

func anyAsValue(any ast.Any) *ast.ImpliedValue {
	return &ast.ImpliedValue{
		Kind: "ImpliedValue",
		Of:   any,
	}
}

func (p *Parser) decorate(any ast.Any, err error) (ast.Value, error) {
	if err != nil {
		return nil, err
	}
	// See if there's a first decorator.
	val, ok, err := p.matchDecorator(any, nil)
	if err != nil {
		return nil, err
	}
	if !ok {
		// No decorator.  Just return the input value.
		return anyAsValue(any), nil
	}
	// Now see if there are additional decorators to apply as casts and
	// return value chain, wrapped if at all, as an ast.Value.
	for {
		outer, ok, err := p.matchDecorator(nil, val)
		if err != nil {
			return nil, err
		}
		if !ok {
			return val, nil
		}
		val = outer
	}
}

// We pass both any and val in here to avoid having to backtrack.
// If we had proper backtracking, this would look a little more sensible.
func (p *Parser) matchDecorator(any ast.Any, val ast.Value) (ast.Value, bool, error) {
	l := p.lexer
	// If there isn't a decorator, just return.  A decorator cannot start
	// with ":::" so we check this condition which arises when an IP6 address or net
	// with a "::" prefix follows a map key (and so we are checking for a decorator
	// here after the key value but before the key colon).
	if lookahead, err := l.peek3(); err != nil || lookahead == "" || lookahead[:2] != "::" || lookahead == ":::" {
		return nil, false, err
	}
	l.skip(2)
	val, err := p.parseDecorator(any, val)
	if noEOF(err) != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (p *Parser) parseDecorator(any ast.Any, val ast.Value) (ast.Value, error) {
	l := p.lexer
	// We can have either:
	//   Case 1: =<name>
	//   Case 2: <type-component> (unions and typedefs with unions must have parens)
	ok, err := l.match('=')
	if err != nil {
		return nil, err
	}
	if ok {
		name, err := l.scanTypeName()
		if name == "" || err != nil {
			return nil, p.error("bad short-form type definition")
		}
		return &ast.DefValue{
			Kind:     "DefValue",
			Of:       any,
			TypeName: name,
		}, nil
	}
	typ, err := p.matchTypeComponent()
	if err != nil {
		return nil, err
	}
	if any != nil {
		return &ast.CastValue{
			Kind: "CastValue",
			Of:   anyAsValue(any),
			Type: typ,
		}, nil
	}
	return &ast.CastValue{
		Kind: "CastValue",
		Of:   val,
		Type: typ,
	}, nil
}

func (p *Parser) matchPrimitive() (*ast.Primitive, error) {
	if val, err := p.matchStringPrimitive(); val != nil || err != nil {
		return val, noEOF(err)
	}
	l := p.lexer
	if err := l.skipSpace(); err != nil {
		return nil, noEOF(err)
	}
	s, err := l.peekPrimitive()
	if err != nil {
		return nil, noEOF(err)
	}
	if s == "" {
		return nil, nil
	}
	// Try to parse the string different ways.  This is not intended
	// to be performant.  CSUP/BSUP provides performance for the Super data model.
	var typ string
	if s == "true" || s == "false" {
		typ = "bool"
	} else if s == "null" {
		typ = "null"
	} else if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		typ = "int64"
	} else if _, err := strconv.ParseUint(s, 10, 64); err == nil {
		typ = "uint64"
	} else if _, err := strconv.ParseFloat(s, 64); err == nil {
		typ = "float64"
	} else if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
		typ = "time"
	} else if _, err := nano.ParseDuration(s); err == nil {
		typ = "duration"
	} else if _, err := netip.ParsePrefix(s); err == nil {
		typ = "net"
	} else if _, err := netip.ParseAddr(s); err == nil {
		typ = "ip"
	} else if len(s) >= 2 && s[0:2] == "0x" {
		if len(s) == 2 {
			typ = "bytes"
		} else if _, err := hex.DecodeString(s[2:]); err == nil {
			typ = "bytes"
		} else {
			return nil, err
		}
	} else {
		// no match
		return nil, nil
	}
	l.skip(len(s))
	return &ast.Primitive{
		Kind: "Primitive",
		Type: typ,
		Text: s,
	}, nil
}

func (p *Parser) matchStringPrimitive() (*ast.Primitive, error) {
	s, ok, err := p.matchString()
	if err != nil || !ok {
		return nil, noEOF(err)
	}
	return &ast.Primitive{
		Kind: "Primitive",
		Type: "string",
		Text: s,
	}, nil
}

func (p *Parser) matchString() (string, bool, error) {
	l := p.lexer
	ok, err := l.match('"')
	if err != nil || !ok {
		return "", false, noEOF(err)
	}
	s, err := l.scanString()
	if err != nil {
		return "", false, p.errorf("string literal: %s", err)
	}
	ok, err = l.match('"')
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, p.error("mismatched string quotes")
	}
	return s, true, nil
}

func (p *Parser) matchRecord() (*ast.Record, error) {
	l := p.lexer
	if ok, err := l.match('{'); !ok || err != nil {
		return nil, noEOF(err)
	}
	fields, err := p.matchFields()
	if err != nil {
		return nil, err
	}
	ok, err := l.match('}')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("mismatched braces while parsing record type")
	}
	return &ast.Record{
		Kind:   "Record",
		Fields: fields,
	}, nil
}

func (p *Parser) matchFields() ([]ast.Field, error) {
	l := p.lexer
	var fields []ast.Field
	seen := make(map[string]struct{})
	for {
		field, err := p.matchField()
		if err != nil {
			return nil, err
		}
		if field == nil {
			break
		}
		if _, ok := seen[field.Name]; !ok {
			fields = append(fields, *field)
		}
		seen[field.Name] = struct{}{}
		ok, err := l.match(',')
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
	}
	return fields, nil
}

func (p *Parser) matchField() (*ast.Field, error) {
	l := p.lexer
	name, ok, err := p.matchSymbol()
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
		return nil, p.errorf("no type name found for field %q", name)
	}
	val, err := p.ParseValue()
	if err != nil {
		return nil, err
	}
	return &ast.Field{
		Name:  name,
		Value: val,
	}, nil
}

func (p *Parser) matchSymbol() (string, bool, error) {
	s, ok, err := p.matchString()
	if err != nil {
		return "", false, noEOF(err)
	}
	if ok {
		return s, true, nil
	}
	s, err = p.matchIdentifier()
	if err != nil || s == "" {
		return "", false, err
	}
	return s, true, nil
}

func (p *Parser) matchArray() (*ast.Array, error) {
	l := p.lexer
	if ok, err := l.match('['); !ok || err != nil {
		return nil, noEOF(err)
	}
	vals, err := p.matchValueList()
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
	return &ast.Array{
		Kind:     "Array",
		Elements: vals,
	}, nil
}

func (p *Parser) matchValueList() ([]ast.Value, error) {
	l := p.lexer
	var vals []ast.Value
	for {
		val, err := p.matchValue()
		if err != nil {
			return nil, err
		}
		if val == nil {
			break
		}
		vals = append(vals, val)
		ok, err := l.match(',')
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
	}
	return vals, nil
}

func (p *Parser) matchSetOrMap() (ast.Any, error) {
	l := p.lexer
	if ok, err := l.match('|'); !ok || err != nil {
		return nil, noEOF(err)
	}
	isSet, err := l.matchTight('[')
	if err != nil {
		return nil, err
	}
	var val ast.Any
	var which string
	if isSet {
		which = "set"
		vals, err := p.matchValueList()
		if err != nil {
			return nil, err
		}
		ok, err := l.match(']')
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.error("mismatched set value brackets")
		}
		val = &ast.Set{
			Kind:     "Set",
			Elements: vals,
		}
	} else {
		ok, err := l.matchTight('{')
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.error("no '|[' or '|{' type bracket at '|' character")
		}
		which = "map"
		entries, err := p.matchMapEntries()
		if err != nil {
			return nil, err
		}
		ok, err = l.match('}')
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.error("mismatched map value brackets")
		}
		val = &ast.Map{
			Kind:    "Map",
			Entries: entries,
		}
	}
	ok, err := l.matchTight('|')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.errorf("mismatched closing bracket while parsing %s value", which)
	}
	return val, nil

}

func (p *Parser) matchMapEntries() ([]ast.Entry, error) {
	var entries []ast.Entry
	for {
		entry, err := p.parseEntry()
		if err != nil {
			return nil, err
		}
		if entry == nil {
			break
		}
		entries = append(entries, *entry)
		ok, err := p.lexer.match(',')
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
	}
	return entries, nil
}

func (p *Parser) parseEntry() (*ast.Entry, error) {
	key, err := p.matchValue()
	if err != nil {
		return nil, err
	}
	if key == nil {
		// no match
		return nil, nil
	}
	ok, err := p.lexer.match(':')
	if err != nil {

		return nil, err
	}
	if !ok {
		return nil, p.error("no colon found after map key while parsing map entry")
	}
	val, err := p.ParseValue()
	if err != nil {
		return nil, err
	}
	return &ast.Entry{
		Key:   key,
		Value: val,
	}, nil
}

func (p *Parser) matchError() (*ast.Error, error) {
	// We only detect identifier-style enum values even though they can
	// also be strings but we don't know that until the semantic check.
	name, err := p.matchIdentifier()
	if err != nil || name != "error" {
		return nil, noEOF(err)
	}
	l := p.lexer
	if ok, err := l.match('('); !ok || err != nil {
		return nil, noEOF(err)
	}
	val, err := p.matchValue()
	if err != nil {
		return nil, noEOF(err)
	}
	if ok, err := l.match(')'); !ok || err != nil {
		return nil, noEOF(err)
	}
	return &ast.Error{
		Kind:  "Error",
		Value: val,
	}, nil
}

func (p *Parser) matchTypeValue() (*ast.TypeValue, error) {
	l := p.lexer
	if ok, err := l.match('<'); !ok || err != nil {
		return nil, noEOF(err)
	}
	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}
	ok, err := l.match('>')
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, p.error("mismatched parentheses while parsing type value")
	}
	return &ast.TypeValue{
		Kind:  "TypeValue",
		Value: typ,
	}, nil
}

func ParsePrimitive(typeText, valText string) (super.Value, error) {
	typ := super.LookupPrimitive(typeText)
	if typ == nil {
		return super.Null, fmt.Errorf("no such type: %s", typeText)
	}
	var b scode.Builder
	if err := BuildPrimitive(&b, Primitive{Type: typ, Text: valText}); err != nil {
		return super.Null, err
	}
	it := b.Bytes().Iter()
	return super.NewValue(typ, it.Next()), nil
}
