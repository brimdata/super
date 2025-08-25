package sup

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/terminal/color"
	"github.com/brimdata/super/scode"
)

type Formatter struct {
	typedefs      map[string]*super.TypeNamed
	permanent     map[string]*super.TypeNamed
	persist       *regexp.Regexp
	tab           int
	newline       string
	builder       strings.Builder
	stack         []strings.Builder
	implied       map[super.Type]bool
	colors        color.Stack
	colorDisabled bool
}

func NewFormatter(pretty int, colorDisabled bool, persist *regexp.Regexp) *Formatter {
	var newline string
	if pretty > 0 {
		newline = "\n"
	}
	var permanent map[string]*super.TypeNamed
	if persist != nil {
		permanent = make(map[string]*super.TypeNamed)
	}
	return &Formatter{
		typedefs:      make(map[string]*super.TypeNamed),
		permanent:     permanent,
		tab:           pretty,
		newline:       newline,
		implied:       make(map[super.Type]bool),
		persist:       persist,
		colorDisabled: colorDisabled,
	}
}

// Persist matches type names to the regular expression provided and
// persists the matched types across records in the stream.  This is useful
// when typedefs have complicated type signatures, e.g., as generated
// by fused fields of records creating a union of records.
func (f *Formatter) Persist(re *regexp.Regexp) {
	f.permanent = make(map[string]*super.TypeNamed)
	f.persist = re
}

func (f *Formatter) push() {
	f.stack = append(f.stack, f.builder)
	f.builder = strings.Builder{}
}

func (f *Formatter) pop() {
	n := len(f.stack)
	f.builder = f.stack[n-1]
	f.stack = f.stack[:n-1]
}

func (f *Formatter) FormatValue(val super.Value) string {
	// We reset tyepdefs so named types are emitted with their
	// definition at first use in each record according to the
	// left-to-right DFS order.  We could make this more efficient
	// by putting a record number/nonce in the map but SUP
	// is already intended to be the low performance path.
	f.typedefs = make(map[string]*super.TypeNamed)
	return f.Format(val)
}

func FormatValue(val super.Value) string {
	return NewFormatter(0, true, nil).Format(val)
}

func String(p any) string {
	if typ, ok := p.(super.Type); ok {
		return FormatType(typ)
	}
	switch val := p.(type) {
	case *super.Value:
		return FormatValue(*val)
	case super.Value:
		return FormatValue(val)
	default:
		panic(fmt.Sprintf("sup.String takes a super.Type or *super.Value: %T", val))
	}
}

func (f *Formatter) Format(val super.Value) string {
	f.builder.Reset()
	f.formatValueAndDecorate(val.Type(), val.Bytes())
	return f.builder.String()
}

func (f *Formatter) hasName(typ super.Type) bool {
	named, ok := typ.(*super.TypeNamed)
	if !ok {
		return false
	}
	if _, ok := f.typedefs[named.Name]; ok {
		return true
	}
	if f.permanent != nil {
		if _, ok = f.permanent[named.Name]; ok {
			return true
		}
	}
	return false
}

func (f *Formatter) nameOf(typ super.Type) string {
	named, ok := typ.(*super.TypeNamed)
	if !ok {
		return ""
	}
	if typ == f.typedefs[named.Name] {
		return named.Name
	}
	if f.permanent != nil {
		if typ == f.permanent[named.Name] {
			return named.Name
		}
	}
	return ""
}

func (f *Formatter) saveType(named *super.TypeNamed) {
	name := named.Name
	f.typedefs[name] = named
	if f.permanent != nil && f.persist.MatchString(name) {
		f.permanent[name] = named
	}
}

func (f *Formatter) formatValueAndDecorate(typ super.Type, bytes scode.Bytes) {
	known := f.hasName(typ)
	implied := f.isImplied(typ)
	f.formatValue(0, typ, bytes, known, implied, false)
	f.decorate(typ, false, bytes == nil)
}

func (f *Formatter) formatValue(indent int, typ super.Type, bytes scode.Bytes, parentKnown, parentImplied, decorate bool) {
	known := parentKnown || f.hasName(typ)
	if bytes == nil {
		f.build("null")
		if parentImplied {
			parentKnown = false
		}
		if decorate {
			f.decorate(typ, parentKnown, true)
		}
		return
	}
	var null bool
	switch t := typ.(type) {
	default:
		f.startColorPrimitive(typ)
		formatPrimitive(&f.builder, typ, bytes)
		f.endColor()
	case *super.TypeNamed:
		f.formatValue(indent, t.Type, bytes, known, parentImplied, false)
	case *super.TypeRecord:
		f.formatRecord(indent, t, bytes, known, parentImplied)
	case *super.TypeArray:
		null = f.formatVector(indent, "[", "]", t.Type, super.NewValue(t, bytes), known, parentImplied)
	case *super.TypeSet:
		null = f.formatVector(indent, "|[", "]|", t.Type, super.NewValue(t, bytes), known, parentImplied)
	case *super.TypeUnion:
		f.formatUnion(indent, t, bytes)
	case *super.TypeMap:
		null = f.formatMap(indent, t, bytes, known, parentImplied)
	case *super.TypeEnum:
		f.build("\"")
		f.build(t.Symbols[super.DecodeUint(bytes)])
		f.build("\"")
	case *super.TypeError:
		f.startColor(color.Red)
		f.build("error")
		f.endColor()
		f.build("(")
		f.formatValue(indent, t.Type, bytes, known, parentImplied, false)
		f.build(")")
	case *super.TypeOfType:
		f.startColor(color.Gray(200))
		f.build("<")
		f.formatTypeValue(indent, bytes, false)
		f.build(">")
		f.endColor()
	}
	if decorate {
		f.decorate(typ, parentKnown, null)
	}
}

func (f *Formatter) formatTypeValue(indent int, tv scode.Bytes, isComponentType bool) scode.Bytes {
	n, tv := super.DecodeLength(tv)
	if tv == nil {
		f.truncTypeValueErr()
		return nil
	}
	switch n {
	default:
		typ, err := super.LookupPrimitiveByID(n)
		if err != nil {
			f.buildf("<ERR bad type ID in type value: %s>", err)
			return nil
		}

		f.startColor(color.Gray(160))
		f.build(super.PrimitiveName(typ))
		f.endColor()
	case super.TypeValueNameDef:
		var name string
		name, tv = super.DecodeName(tv)
		if tv == nil {
			f.truncTypeValueErr()
			return nil
		}
		if isComponentType {
			f.build("(")
		}
		f.build(QuotedName(name))
		f.build("=")
		tv = f.formatTypeValue(indent, tv, false)
		if isComponentType {
			f.build(")")
		}
	case super.TypeValueNameRef:
		var name string
		name, tv = super.DecodeName(tv)
		if tv == nil {
			f.truncTypeValueErr()
			return nil
		}
		f.build(QuotedName(name))
	case super.TypeValueRecord:
		f.build("{")
		var n int
		n, tv = super.DecodeLength(tv)
		if tv == nil {
			f.truncTypeValueErr()
			return nil
		}
		if n == 0 {
			f.build("}")
			return tv
		}
		sep := f.newline
		indent += f.tab
		for range n {
			f.build(sep)
			var name string
			name, tv = super.DecodeName(tv)
			if tv == nil {
				f.truncTypeValueErr()
				return nil
			}
			f.indent(indent, QuotedName(name))
			f.build(":")
			if f.tab > 0 {
				f.build(" ")
			}
			tv = f.formatTypeValue(indent, tv, false)
			sep = "," + f.newline
		}
		f.build(f.newline)
		f.indent(indent-f.tab, "}")
	case super.TypeValueArray:
		tv = f.formatVectorTypeValue(indent, "[", "]", tv)
	case super.TypeValueSet:
		tv = f.formatVectorTypeValue(indent, "|[", "]|", tv)
	case super.TypeValueMap:
		f.build("|{")
		newline := f.newline
		indent += f.tab
		if n, itv := super.DecodeLength(tv); n < super.IDTypeComplex {
			n, _ = super.DecodeLength(itv)
			if n < super.IDTypeComplex {
				// If key and value are both primitives don't indent.
				indent -= f.tab
				newline = ""
			}
		}
		f.build(newline)
		f.indent(indent, "")
		tv = f.formatTypeValue(indent, tv, false)
		f.build(":")
		if f.tab > 0 {
			f.build(" ")
		}
		tv = f.formatTypeValue(indent, tv, false)
		f.build(newline)
		if newline != "" {
			f.indent(indent-f.tab, "}|")
		} else {
			f.build("}|")
		}
	case super.TypeValueUnion:
		var n int
		n, tv = super.DecodeLength(tv)
		if tv == nil {
			f.truncTypeValueErr()
			return nil
		}
		indent += f.tab
		if isComponentType {
			f.build("(" + f.newline)
		} else {
			f.build(f.newline)
		}
		f.indent(indent, "")
		for k := range n {
			tv = f.formatTypeValue(indent, tv, true)
			if k != n-1 {
				f.build("|" + f.newline)
				f.indent(indent, "")
			}
		}
		f.build(f.newline)
		indent -= f.tab
		if isComponentType {
			f.indent(indent, ")")
		} else {
			f.indent(indent, "")
		}
	case super.TypeValueEnum:
		f.build("enum(")
		var n int
		n, tv = super.DecodeLength(tv)
		if tv == nil {
			f.truncTypeValueErr()
			return nil
		}
		for k := range n {
			if k > 0 {
				f.build(",")
			}
			var symbol string
			symbol, tv = super.DecodeName(tv)
			if tv == nil {
				f.truncTypeValueErr()
				return nil
			}
			f.build(QuotedName(symbol))
		}
		f.build(")")
	case super.TypeValueError:
		f.startColor(color.Red)
		f.build("error")
		f.endColor()
		f.build("(")
		tv = f.formatTypeValue(indent, tv, false)
		f.build(")")
	}
	return tv
}

func (f *Formatter) formatVectorTypeValue(indent int, open, close string, tv scode.Bytes) scode.Bytes {
	f.build(open)
	if n, _ := super.DecodeLength(tv); n < super.IDTypeComplex {
		tv = f.formatTypeValue(indent, tv, false)
		f.build(close)
		return tv
	}
	indent += f.tab
	f.build(f.newline)
	f.indent(indent, "")
	tv = f.formatTypeValue(indent, tv, false)
	f.build(f.newline)
	f.indent(indent-f.tab, close)
	return tv
}

func (f *Formatter) truncTypeValueErr() {
	f.build("<ERR truncated type value>")
}

func (f *Formatter) decorate(typ super.Type, known, null bool) {
	if known || (!(null && typ != super.TypeNull) && f.isImplied(typ)) {
		return
	}
	f.startColor(color.Gray(200))
	defer f.endColor()
	if name := f.nameOf(typ); name != "" {
		f.buildf("::%s", quoteHexyString(QuotedTypeName(name)))
	} else if SelfDescribing(typ) && !null {
		if typ, ok := typ.(*super.TypeNamed); ok {
			f.saveType(typ)
			f.buildf("::=%s", QuotedTypeName(typ.Name))
		}
	} else {
		f.build("::")
		f.formatType(typ, true)
	}
}

func (f *Formatter) formatRecord(indent int, typ *super.TypeRecord, bytes scode.Bytes, known, parentImplied bool) {
	f.build("{")
	if len(typ.Fields) == 0 {
		f.build("}")
		return
	}
	indent += f.tab
	sep := f.newline
	it := bytes.Iter()
	for _, field := range typ.Fields {
		f.build(sep)
		f.startColor(color.Blue)
		f.indent(indent, QuotedName(field.Name))
		f.endColor()
		f.build(":")
		if f.tab > 0 {
			f.build(" ")
		}
		f.formatValue(indent, field.Type, it.Next(), known, parentImplied, true)
		sep = "," + f.newline
	}
	f.build(f.newline)
	f.indent(indent-f.tab, "}")
}

func (f *Formatter) formatVector(indent int, open, close string, inner super.Type, val super.Value, known, parentImplied bool) bool {
	f.build(open)
	n, err := val.ContainerLength()
	if err != nil {
		panic(err)
	}
	if n == 0 {
		f.build(close)
		return true
	}
	indent += f.tab
	sep := f.newline
	it := val.Iter()
	elems := newElemBuilder(inner)
	for !it.Done() {
		f.build(sep)
		f.indent(indent, "")
		typ, b := elems.add(it.Next())
		f.formatValue(indent, typ, b, known, parentImplied, true)
		sep = "," + f.newline
	}
	f.build(f.newline)
	f.indent(indent-f.tab, close)
	if elems.needsDecoration() {
		// If we haven't seen all the types in the union, print the decorator
		// so the fullness of the union is persevered.
		f.decorate(val.Type(), false, true)
	}
	return false
}

type elemHelper struct {
	typ   super.Type
	union *super.TypeUnion
	seen  map[super.Type]struct{}
}

func newElemBuilder(typ super.Type) *elemHelper {
	union, _ := super.TypeUnder(typ).(*super.TypeUnion)
	return &elemHelper{typ: typ, union: union, seen: make(map[super.Type]struct{})}
}

func (e *elemHelper) add(b scode.Bytes) (super.Type, scode.Bytes) {
	if e.union == nil {
		return e.typ, b
	}
	if b == nil {
		// The type returned from union.SplitBSUP for a null value will
		// be the union type. While this is the correct type, for
		// display purposes we do not want to see the decorator so just
		// set the type to null.
		return super.TypeNull, b
	}
	typ, b := e.union.Untag(b)
	if _, ok := e.seen[typ]; !ok {
		e.seen[typ] = struct{}{}
	}
	return typ, b
}

func (e *elemHelper) needsDecoration() bool {
	_, isnamed := e.typ.(*super.TypeNamed)
	return e.union != nil && (isnamed || len(e.seen) < len(e.union.Types))
}

func (f *Formatter) formatUnion(indent int, union *super.TypeUnion, bytes scode.Bytes) {
	typ, bytes := union.Untag(bytes)
	// XXX For now, we always decorate a union value so that
	// we can determine the tag from the value's explicit type.
	// We can later optimize this so we only print the decorator if its
	// ambigous with another type (e.g., int8 and int16 vs a union of int8 and string).
	// Let's do this after we have the parser working and capable of this
	// disambiguation.  See issue #1764.
	// In other words, just because we known the union's type doesn't mean
	// we know the type of a particular value of that union.
	const known = false
	const parentImplied = true
	f.formatValue(indent, typ, bytes, known, parentImplied, true)
}

func (f *Formatter) formatMap(indent int, typ *super.TypeMap, bytes scode.Bytes, known, parentImplied bool) bool {
	empty := true
	f.build("|{")
	indent += f.tab
	sep := f.newline
	keyElems := newElemBuilder(typ.KeyType)
	valElems := newElemBuilder(typ.ValType)
	for it := bytes.Iter(); !it.Done(); {
		keyBytes := it.Next()
		empty = false
		f.build(sep)
		f.indent(indent, "")
		var keyType super.Type
		keyType, keyBytes = keyElems.add(keyBytes)
		f.formatValue(indent, keyType, keyBytes, known, parentImplied, true)
		if super.TypeUnder(keyType) == super.TypeIP && len(keyBytes) == 16 {
			// To avoid ambiguity, whitespace must separate an IPv6
			// map key from the colon that follows it.
			f.build(" ")
		}
		f.build(":")
		if f.tab > 0 {
			f.build(" ")
		}
		valType, valBytes := valElems.add(it.Next())
		f.formatValue(indent, valType, valBytes, known, parentImplied, true)
		sep = "," + f.newline
	}
	f.build(f.newline)
	f.indent(indent-f.tab, "}|")
	if keyElems.needsDecoration() || valElems.needsDecoration() {
		f.decorate(typ, false, true)
	}
	return empty
}

func (f *Formatter) indent(tab int, s string) {
	for range tab {
		f.builder.WriteByte(' ')
	}
	f.build(s)
}

func (f *Formatter) build(s string) {
	f.builder.WriteString(s)
}

func (f *Formatter) buildf(s string, args ...any) {
	f.builder.WriteString(fmt.Sprintf(s, args...))
}

// formatType builds typ as a type string with any needed
// typedefs for named types that have not been previously defined,
// or whose name is redefined to a different type.
// These typedefs use the embedded syntax (name=type-string).
// Typedefs handled by decorators are handled in decorate().
// The routine re-enters the type formatter with a fresh builder by
// invoking push()/pop().
func (f *Formatter) formatType(typ super.Type, isComponentType bool) {
	if name := f.nameOf(typ); name != "" {
		f.build(name)
		return
	}
	if named, ok := typ.(*super.TypeNamed); ok {
		needParens := isComponentType || super.IsUnionType(named.Type)
		f.saveType(named)
		if needParens {
			f.build("(")
		}
		f.build(named.Name)
		f.build("=")
		f.formatType(named.Type, false)
		if needParens {
			f.build(")")
		}
		return
	}
	if typ.ID() < super.IDTypeComplex {
		f.build(super.PrimitiveName(typ))
		return
	}
	f.push()
	f.formatTypeBody(typ, isComponentType)
	s := f.builder.String()
	f.pop()
	f.build(s)
}

func (f *Formatter) formatTypeBody(typ super.Type, isComponentType bool) {
	if name := f.nameOf(typ); name != "" {
		f.build(name)
		return
	}
	switch typ := typ.(type) {
	case *super.TypeNamed:
		// Named types are handled differently above to determine the
		// plain form vs embedded typedef.
		panic("named type shouldn't be formatted")
	case *super.TypeRecord:
		f.formatTypeRecord(typ)
	case *super.TypeArray:
		f.build("[")
		f.formatType(typ.Type, false)
		f.build("]")
	case *super.TypeSet:
		f.build("|[")
		f.formatType(typ.Type, false)
		f.build("]|")
	case *super.TypeMap:
		f.build("|{")
		f.formatType(typ.KeyType, false)
		f.build(":")
		f.formatType(typ.ValType, false)
		f.build("}|")
	case *super.TypeUnion:
		f.formatTypeUnion(typ, isComponentType)
	case *super.TypeEnum:
		f.formatTypeEnum(typ)
	case *super.TypeError:
		f.build("error(")
		formatType(&f.builder, make(map[string]*super.TypeNamed), typ.Type, false)
		f.build(")")
	case *super.TypeOfType:
		formatType(&f.builder, make(map[string]*super.TypeNamed), typ, false)
	default:
		panic("unknown case in formatTypeBody: " + String(typ))
	}
}

func (f *Formatter) formatTypeRecord(typ *super.TypeRecord) {
	f.build("{")
	for k, field := range typ.Fields {
		if k > 0 {
			f.build(",")
		}
		f.build(QuotedName(field.Name))
		f.build(":")
		f.formatType(field.Type, false)
	}
	f.build("}")
}

func (f *Formatter) formatTypeUnion(typ *super.TypeUnion, isComponentType bool) {
	if isComponentType {
		f.build("(")
	}
	for k, typ := range typ.Types {
		if k > 0 {
			f.build("|")
		}
		f.formatType(typ, true)
	}
	if isComponentType {
		f.build(")")
	}
}

func (f *Formatter) formatTypeEnum(typ *super.TypeEnum) {
	f.build("enum(")
	for k, s := range typ.Symbols {
		if k > 0 {
			f.build(",")
		}
		f.buildf("%s", QuotedName(s))
	}
	f.build(")")
}

var colors = map[super.Type]color.Code{
	super.TypeString: color.Green,
	super.TypeType:   color.Orange,
}

func (f *Formatter) startColorPrimitive(typ super.Type) {
	if !f.colorDisabled {
		c, ok := colors[super.TypeUnder(typ)]
		if !ok {
			c = color.Reset
		}
		f.startColor(c)
	}
}

func (f *Formatter) startColor(code color.Code) {
	if !f.colorDisabled {
		f.colors.Start(&f.builder, code)
	}
}

func (f *Formatter) endColor() {
	if !f.colorDisabled {
		f.colors.End(&f.builder)
	}
}

func (f *Formatter) isImplied(typ super.Type) bool {
	implied, ok := f.implied[typ]
	if !ok {
		implied = Implied(typ)
		f.implied[typ] = implied
	}
	return implied
}

// FormatType formats a type in canonical form to represent type values
// as standalone entities.
func FormatType(typ super.Type) string {
	var b strings.Builder
	formatType(&b, make(map[string]*super.TypeNamed), typ, false)
	return b.String()
}

func formatType(b *strings.Builder, typedefs map[string]*super.TypeNamed, typ super.Type, isComponentType bool) {
	switch t := typ.(type) {
	case *super.TypeNamed:
		name := t.Name
		b.WriteString(QuotedTypeName(name))
		if typedefs[t.Name] != t {
			b.WriteByte('=')
			formatType(b, typedefs, t.Type, false)
			// Don't set typedef until after children are recursively
			// traversed so that we adhere to the DFS order of
			// type bindings.
			typedefs[name] = t
		}
	case *super.TypeRecord:
		b.WriteByte('{')
		for k, f := range t.Fields {
			if k > 0 {
				b.WriteByte(',')
			}
			b.WriteString(QuotedName(f.Name))
			b.WriteString(":")
			formatType(b, typedefs, f.Type, false)
		}
		b.WriteByte('}')
	case *super.TypeArray:
		b.WriteByte('[')
		formatType(b, typedefs, t.Type, false)
		b.WriteByte(']')
	case *super.TypeSet:
		b.WriteString("|[")
		formatType(b, typedefs, t.Type, false)
		b.WriteString("]|")
	case *super.TypeMap:
		b.WriteString("|{")
		formatType(b, typedefs, t.KeyType, false)
		b.WriteByte(':')
		formatType(b, typedefs, t.ValType, false)
		b.WriteString("}|")
	case *super.TypeUnion:
		if isComponentType {
			b.WriteByte('(')
		}
		for k, typ := range t.Types {
			if k > 0 {
				b.WriteByte('|')
			}
			formatType(b, typedefs, typ, true)
		}
		if isComponentType {
			b.WriteByte(')')
		}
	case *super.TypeEnum:
		b.WriteString("enum(")
		for k, s := range t.Symbols {
			if k > 0 {
				b.WriteByte(',')
			}
			b.WriteString(QuotedName(s))
		}
		b.WriteByte(')')
	case *super.TypeError:
		b.WriteString("error(")
		formatType(b, typedefs, t.Type, false)
		b.WriteByte(')')
	default:
		b.WriteString(super.PrimitiveName(typ))
	}
}

func FormatPrimitive(typ super.Type, bytes scode.Bytes) string {
	var b strings.Builder
	formatPrimitive(&b, typ, bytes)
	return b.String()
}

func formatPrimitive(b *strings.Builder, typ super.Type, bytes scode.Bytes) {
	if bytes == nil {
		b.WriteString("null")
		return
	}
	switch typ := typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		b.WriteString(strconv.FormatUint(super.DecodeUint(bytes), 10))
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64:
		b.WriteString(strconv.FormatInt(super.DecodeInt(bytes), 10))
	case *super.TypeOfDuration:
		b.WriteString(super.DecodeDuration(bytes).String())
	case *super.TypeOfTime:
		b.WriteString(super.DecodeTime(bytes).Time().Format(time.RFC3339Nano))
	case *super.TypeOfFloat16:
		f := super.DecodeFloat16(bytes)
		if f == float32(int64(f)) {
			b.WriteString(fmt.Sprintf("%d.", int64(f)))
		} else {
			b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
		}
	case *super.TypeOfFloat32:
		f := super.DecodeFloat32(bytes)
		if f == float32(int64(f)) {
			b.WriteString(fmt.Sprintf("%d.", int64(f)))
		} else {
			b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
		}
	case *super.TypeOfFloat64:
		f := super.DecodeFloat64(bytes)
		if f == float64(int64(f)) {
			b.WriteString(fmt.Sprintf("%d.", int64(f)))
		} else {
			b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		}
	case *super.TypeOfBool:
		if super.DecodeBool(bytes) {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case *super.TypeOfBytes:
		b.WriteString("0x")
		b.WriteString(hex.EncodeToString(bytes))
	case *super.TypeOfString:
		b.WriteString(QuotedString(string(bytes)))
	case *super.TypeOfIP:
		b.WriteString(super.DecodeIP(bytes).String())
	case *super.TypeOfNet:
		b.WriteString(super.DecodeNet(bytes).String())
	case *super.TypeOfType:
		b.WriteByte('<')
		b.WriteString(FormatTypeValue(bytes))
		b.WriteByte('>')
	default:
		b.WriteString(fmt.Sprintf("<SUP unknown primitive: %T>", typ))
	}
}

func FormatTypeValue(tv scode.Bytes) string {
	f := NewFormatter(0, true, nil)
	f.formatTypeValue(0, tv, false)
	return f.builder.String()
}

func quoteHexyString(s string) string {
	if s == "" || len(s) > 4 {
		return s
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || (c >= '0' && c <= '9')) {
			return s
		}
	}
	return fmt.Sprintf("%q", s)
}
