package zeekio

import (
	"bytes"
	"errors"
	"net/netip"
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/scode"
	"golang.org/x/text/unicode/norm"
)

type builder struct {
	scode.Builder
	buf             []byte
	fields          [][]byte
	reorderedFields [][]byte
	val             super.Value
}

func (b *builder) build(typ *super.TypeRecord, sourceFields []int, path []byte, data []byte) (*super.Value, error) {
	b.Truncate()
	b.Grow(len(data))
	fields := typ.Fields
	if path != nil {
		if fields[0].Name != "_path" {
			return nil, errors.New("no _path in field 0")
		}
		fields = fields[1:]
		b.Append(path)
	}
	b.fields = b.fields[:0]
	var start int

	const separator = '\t'

	for i, c := range data {
		if c == separator {
			b.fields = append(b.fields, data[start:i])
			start = i + 1
		}
	}
	b.fields = append(b.fields, data[start:])
	if actual, expected := len(b.fields), len(sourceFields); actual > expected {
		return nil, errors.New("too many values")
	} else if actual < expected {
		return nil, errors.New("too few values")
	}
	b.reorderedFields = b.reorderedFields[:0]
	for _, s := range sourceFields {
		b.reorderedFields = append(b.reorderedFields, b.fields[s])
	}
	leftoverFields, err := b.appendFields(fields, b.reorderedFields)
	if err != nil {
		return nil, err
	}
	if len(leftoverFields) != 0 {
		return nil, errors.New("too many values")
	}
	b.val = super.NewValue(typ, b.Bytes())
	return &b.val, nil
}

func (b *builder) appendFields(fields []super.Field, values [][]byte) ([][]byte, error) {
	const setSeparator = ','
	const emptyContainer = "(empty)"
	for _, f := range fields {
		if len(values) == 0 {
			return nil, errors.New("too few values")
		}
		switch typ := f.Type.(type) {
		case *super.TypeArray, *super.TypeSet:
			val := values[0]
			values = values[1:]
			if string(val) == "-" {
				b.Append(nil)
				continue
			}
			b.BeginContainer()
			if bytes.Equal(val, []byte(emptyContainer)) {
				b.EndContainer()
				continue
			}
			inner := super.InnerType(typ)
			var cstart int
			for i, ch := range val {
				if ch == setSeparator {
					if err := b.appendPrimitive(inner, val[cstart:i]); err != nil {
						return nil, err
					}
					cstart = i + 1
				}
			}
			if err := b.appendPrimitive(inner, val[cstart:]); err != nil {
				return nil, err
			}
			if _, ok := typ.(*super.TypeSet); ok {
				b.TransformContainer(super.NormalizeSet)
			}
			b.EndContainer()
		case *super.TypeRecord:
			b.BeginContainer()
			var err error
			if values, err = b.appendFields(typ.Fields, values); err != nil {
				return nil, err
			}
			b.EndContainer()
		default:
			if err := b.appendPrimitive(f.Type, values[0]); err != nil {
				return nil, err
			}
			values = values[1:]
		}
	}
	return values, nil
}

func (b *builder) appendPrimitive(typ super.Type, val []byte) error {
	if string(val) == "-" {
		b.Append(nil)
		return nil
	}
	switch typ.ID() {
	case super.IDInt64:
		v, err := byteconv.ParseInt64(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendInt(b.buf[:0], v)
	case super.IDUint16:
		// Zeek's port type is mapped to a uint16 named type.
		v, err := byteconv.ParseUint16(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendUint(b.buf[:0], uint64(v))
	case super.IDUint64:
		v, err := byteconv.ParseUint64(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendUint(b.buf[:0], v)
	case super.IDDuration:
		v, err := parseTime(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendDuration(b.buf[:0], nano.Duration(v))
	case super.IDTime:
		v, err := parseTime(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendTime(b.buf[:0], v)
	case super.IDFloat64:
		v, err := byteconv.ParseFloat64(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendFloat64(b.buf[:0], v)
	case super.IDBool:
		v, err := byteconv.ParseBool(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendBool(b.buf[:0], v)
	case super.IDString:
		// Zeek's enum type is mapped to string named type.
		val = unescapeZeekString(val)
		if !utf8.Valid(val) {
			// Zeek has an unusual escaping model for non-valid UTF
			// strings in their JSON integration: invalid bytes are
			// formatted as the sequence '\' 'x' h h to indicate
			// the presence of unexpected, invalid binary data where
			// a string was expeceted, e.g., in a field of data coming
			// off the network.  This is a reasonable scheme; however,
			// they don't also escape the sequence `\` `x` if it
			// happens to be in the data, so there is no way to distinguish
			// whether the data was originally in the network or was
			// escaped.  The proper way to handle all this
			// would be for Zeek's logging system to identify these
			// quasi-strings natively (e.g., as a Zed union (string,bytes)),
			// but the Zeek team didn't seem to accept this as a priority,
			// so we simply replicate here what Zeek does for JSON.
			// If there ever is interest, we could create the (strings,bytes)
			// union here, but given the current code structure, which
			// assumes a fixed record-type per log type, it is a little
			// bit involved.  Since the Zeek team doesn't think this is
			// important, we will let this be.
			val = EscapeZeekHex(val)
		}
		b.Append(norm.NFC.Bytes(val))
		return nil
	case super.IDIP:
		v, err := byteconv.ParseIP(val)
		if err != nil {
			return err
		}
		b.buf = super.AppendIP(b.buf[:0], v)
	case super.IDNet:
		v, err := netip.ParsePrefix(string(val))
		if err != nil {
			return err
		}
		b.buf = super.AppendNet(b.buf[:0], v)
	default:
		panic(typ)
	}
	b.Append(b.buf)
	return nil
}
