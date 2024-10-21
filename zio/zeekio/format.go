package zeekio

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zson"
)

func formatAny(val super.Value, inContainer bool) string {
	switch t := val.Type().(type) {
	case *super.TypeArray:
		return formatArray(t, val.Bytes())
	case *super.TypeNamed:
		return formatAny(super.NewValue(t.Type, val.Bytes()), inContainer)
	case *super.TypeOfBool:
		if val.Bool() {
			return "T"
		}
		return "F"
	case *super.TypeOfBytes:
		return base64.StdEncoding.EncodeToString(val.Bytes())
	case *super.TypeOfDuration, *super.TypeOfTime:
		return formatTime(nano.Ts(val.Int()))
	case *super.TypeEnum:
		return formatAny(super.NewValue(super.TypeUint64, val.Bytes()), false)
	case *super.TypeOfFloat16, *super.TypeOfFloat32:
		return strconv.FormatFloat(val.Float(), 'f', -1, 32)
	case *super.TypeOfFloat64:
		return strconv.FormatFloat(val.Float(), 'f', -1, 64)
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64:
		return strconv.FormatInt(val.Int(), 10)
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		return strconv.FormatUint(val.Uint(), 10)
	case *super.TypeOfIP:
		return super.DecodeIP(val.Bytes()).String()
	case *super.TypeMap:
		return formatMap(t, val.Bytes())
	case *super.TypeOfNet:
		return super.DecodeNet(val.Bytes()).String()
	case *super.TypeOfNull:
		return "-"
	case *super.TypeRecord:
		return formatRecord(t, val.Bytes())
	case *super.TypeSet:
		return formatSet(t, val.Bytes())
	case *super.TypeOfString:
		return formatString(t, val.Bytes(), inContainer)
	case *super.TypeOfType:
		return zson.String(val)
	case *super.TypeUnion:
		return formatUnion(t, val.Bytes())
	case *super.TypeError:
		if super.TypeUnder(t.Type) == super.TypeString {
			return string(val.Bytes())
		}
		return zson.FormatValue(val)
	default:
		return fmt.Sprintf("zeekio.StringOf(): unknown type: %T", t)
	}
}

func formatArray(t *super.TypeArray, zv zcode.Bytes) string {
	if len(zv) == 0 {
		return "(empty)"
	}

	var b strings.Builder
	separator := byte(',')

	first := true
	it := zv.Iter()
	for !it.Done() {
		if first {
			first = false
		} else {
			b.WriteByte(separator)
		}
		if val := it.Next(); val == nil {
			b.WriteByte('-')
		} else {
			b.WriteString(formatAny(super.NewValue(t.Type, val), true))
		}
	}
	return b.String()
}

func formatMap(t *super.TypeMap, zv zcode.Bytes) string {
	var b strings.Builder
	it := zv.Iter()
	b.WriteByte('[')
	for !it.Done() {
		b.WriteString(formatAny(super.NewValue(t.KeyType, it.Next()), true))
		b.WriteString(formatAny(super.NewValue(t.ValType, it.Next()), true))
	}
	b.WriteByte(']')
	return b.String()
}

func formatRecord(t *super.TypeRecord, zv zcode.Bytes) string {
	var b strings.Builder
	separator := byte(',')
	first := true
	it := zv.Iter()
	for _, f := range t.Fields {
		if first {
			first = false
		} else {
			b.WriteByte(separator)
		}
		if val := it.Next(); val == nil {
			b.WriteByte('-')
		} else {
			b.WriteString(formatAny(super.NewValue(f.Type, val), false))
		}
	}
	return b.String()
}

func formatSet(t *super.TypeSet, zv zcode.Bytes) string {
	if len(zv) == 0 {
		return "(empty)"
	}
	var b strings.Builder
	separator := byte(',')
	first := true
	it := zv.Iter()
	for !it.Done() {
		if first {
			first = false
		} else {
			b.WriteByte(separator)
		}
		b.WriteString(formatAny(super.NewValue(t.Type, it.Next()), true))
	}
	return b.String()
}

func formatString(t *super.TypeOfString, zv zcode.Bytes, inContainer bool) string {
	if bytes.Equal(zv, []byte{'-'}) {
		return "\\x2d"
	}
	if string(zv) == "(empty)" {
		return "\\x28empty)"
	}
	var out []byte
	var start int
	for i := 0; i < len(zv); {
		r, l := utf8.DecodeRune(zv[i:])
		if r == '\\' {
			out = append(out, zv[start:i]...)
			out = append(out, '\\', '\\')
			i++
			start = i
			continue
		}
		if !unicode.IsPrint(r) || shouldEscape(r, inContainer) {
			out = append(out, zv[start:i]...)
			out = append(out, unescape(r)...)
			i += l
			start = i
		} else {
			i += l
		}
	}
	return string(append(out, zv[start:]...))
}

func unescape(r rune) []byte {
	code := strconv.FormatInt(int64(r), 16)
	n := len(code)
	if (n & 1) != 0 {
		n++
		code = "0" + code
	}
	var b bytes.Buffer
	for k := 0; k < n; k += 2 {
		b.WriteString("\\x")
		b.WriteString(code[k : k+2])
	}
	return b.Bytes()
}

func formatUnion(t *super.TypeUnion, zv zcode.Bytes) string {
	if zv == nil {
		return FormatValue(super.Null)
	}
	typ, iv := t.Untag(zv)
	s := strconv.FormatInt(int64(t.TagOf(typ)), 10) + ":"
	return s + formatAny(super.NewValue(typ, iv), false)
}

func FormatValue(v super.Value) string {
	if v.IsNull() {
		return "-"
	}
	return formatAny(v, false)
}
