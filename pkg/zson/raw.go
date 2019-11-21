package zson

import (
	"errors"
	"fmt"

	"github.com/buger/jsonparser"
	"github.com/mccanne/zq/pkg/nano"
	"github.com/mccanne/zq/pkg/zeek"
	"github.com/mccanne/zq/pkg/zval"
)

// Raw is the serialization format for zson records.  A raw value comprises a
// sequence of zvals, one per descriptor column.  The descriptor is stored
// outside of the raw serialization but is needed to interpret the raw values.
type Raw []byte

// ZvalIter returns an iterator over the receiver's zvals.
func (r Raw) ZvalIter() zval.Iter {
	return zval.Iter(r)
}

// NewRawFromZvals builds a raw value from a descriptor and zvals.
func NewRawFromZvals(d *Descriptor, vals [][]byte) (Raw, error) {
	if nv, nc := len(vals), len(d.Type.Columns); nv != nc {
		return nil, fmt.Errorf("got %d values (%q), expected %d (%q)", nv, vals, nc, d.Type.Columns)

	}
	var raw Raw
	for _, val := range vals {
		raw = zval.AppendValue(raw, val)
	}
	return raw, nil
}

// NewRawAndTsFromJSON builds a raw value from a descriptor and the JSON object
// in data.  It works in two steps.  First, it constructs a slice of views onto
// the underlying JSON values.  This slice follows the order of the descriptor
// columns.  Second, it appends the descriptor ID and the values to a new
// buffer.
func NewRawAndTsFromJSON(d *Descriptor, tsCol int, data []byte) (Raw, nano.Ts, error) {
	type jsonVal struct {
		val []byte
		typ jsonparser.ValueType
	}
	jsonVals := make([]jsonVal, 32) // Fixed size for stack allocation.
	if len(d.Type.Columns) > 32 {
		jsonVals = make([]jsonVal, len(d.Type.Columns))
	}
	n := 2 // Estimate for descriptor ID uvarint.
	callback := func(key []byte, val []byte, typ jsonparser.ValueType, offset int) error {
		if col, ok := d.ColumnOfField(string(key)); ok {
			jsonVals[col] = jsonVal{val, typ}
			n += len(val) + 1 // Estimate for zval and its length uvarint.
		}
		return nil
	}
	if err := jsonparser.ObjectEach(data, callback); err != nil {
		return nil, 0, err
	}
	raw := make([]byte, 0, n)
	var ts nano.Ts
	for i := range d.Type.Columns {
		val := jsonVals[i].val
		if i == tsCol {
			var err error
			ts, err = nano.Parse(val)
			if err != nil {
				ts, err = nano.ParseRFC3339Nano(val)
				if err != nil {
					return nil, 0, err
				}
			}
		}
		switch jsonVals[i].typ {
		case jsonparser.Array:
			vals := make([][]byte, 0, 8) // Fixed size for stack allocation.
			callback := func(v []byte, typ jsonparser.ValueType, offset int, err error) {
				vals = append(vals, v)
			}
			if _, err := jsonparser.ArrayEach(val, callback); err != nil {
				return nil, 0, err
			}
			raw = zval.AppendContainer(raw, vals)
			continue
		case jsonparser.Boolean:
			val = []byte{'F'}
			if val[0] == 't' {
				val = []byte{'T'}
			}
		case jsonparser.Null:
			val = nil
		case jsonparser.String:
			val = zeek.Unescape(val)
		}
		raw = zval.AppendValue(raw, val)
	}
	return raw, ts, nil
}

func NewRawAndTsFromZeekTSV(d *Descriptor, path []byte, data []byte) (Raw, nano.Ts, error) {
	raw := make([]byte, 0)
	columns := d.Type.Columns
	col := 0
	// XXX assert that columns[col].Name == "_path" ?
	raw = appendZvalFromZeek(raw, columns[col].Type, path)
	col++

	var ts nano.Ts
	const separator = '\t'
	var start int
	var nested [][]byte
	handleVal := func(val []byte) error {
		if col >= len(columns) {
			return errors.New("too many values")
		}

		typ := columns[col].Type
		recType, isRec := typ.(*zeek.TypeRecord)
		if isRec {
			if nested == nil {
				nested = make([][]byte, 0)
			}
			nested = append(nested, val)
			if len(nested) == len(recType.Columns) {
				raw = zval.AppendContainer(raw, nested)
				nested = nil
				col++
			}
		} else {
			if columns[col].Name == "ts" {
				var err error
				ts, err = nano.Parse(val)
				if err != nil {
					return err
				}
			}
			raw = appendZvalFromZeek(raw, typ, val)
			col++
		}
		return nil
	}
	
	for i, c := range data {
		if c == separator {
			err := handleVal(data[start:i])
			if err != nil {
				return nil, 0, err
			}
			start = i + 1
		}
	}
	err := handleVal(data[start:])
	if err != nil {
		return nil, 0, err
	}

	if col != len(d.Type.Columns) {
		return nil, 0, errors.New("too few values")
	}
	return raw, ts, nil
}

func NewRawAndTsFromZeekValues(d *Descriptor, tsCol int, vals [][]byte) (Raw, nano.Ts, error) {
	if nv, nc := len(vals), len(d.Type.Columns); nv != nc {
		// Don't pass vals to fmt.Errorf or it will escape to the heap.
		return nil, 0, fmt.Errorf("got %d values, expected %d", nv, nc)
	}
	n := 2 // Estimate for descriptor ID uvarint.
	for _, v := range vals {
		n += len(v) + 1 // Estimate for zval and its length uvarint.
	}
	raw := make([]byte, 0, n)
	var ts nano.Ts
	for i, val := range vals {
		var err error
		if i == tsCol {
			ts, err = nano.Parse(val)
			if err != nil {
				return nil, 0, err
			}
		}
		raw = appendZvalFromZeek(raw, d.Type.Columns[i].Type, val)
	}
	return raw, ts, nil
}

var (
	ErrUnterminated = errors.New("zson parse error: unterminated container")
	ErrSyntax       = errors.New("zson syntax error")
)

func NewRawFromZSON(desc *Descriptor, zson []byte) (Raw, error) {
	fmt.Println("NEW RAW", string(zson))
	// XXX no validation on types from the descriptor, though we'll
	// want to add that to support eg the bytes type.
	// if we did this, we could also get at the ts field without
	// making a separate pass in the parser.
	container, rest, err := zsonParseContainer(zson)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, ErrSyntax
	}
	it := zval.Iter(container)
	if it.Done() {
		return nil, ErrSyntax
	}
	v, isContainer, err := it.Next()
	if err != nil {
		return nil, err
	}
	if !isContainer {
		return nil, ErrSyntax
	}
	return v, nil
	/*
		fmt.Println("TOP CONTAINER", container)
		// XXX the zval API makes this inefficient... we should rework this.
		/*var raw []byte
		for k, v := range vals {
			fmt.Println("RAW", k, Raw(raw).String())
			raw = append(raw, v...)
		}*/
	//	return container, nil
}

const (
	semicolon    = byte(';')
	leftbracket  = byte('[')
	rightbracket = byte(']')
	backslash    = byte('\\')
)

// zsonParseContainer() parses the given byte array representing a container
// in the zson format.
// If there is no error, the first two return values are:
//  1. an array of zvals corresponding to the indivdiual elements
//  2. the passed-in byte array advanced past all the data that was parsed.
func zsonParseContainer(b []byte) (Raw, []byte, error) {
	// skip leftbracket
	b = b[1:]

	// XXX if we have the Type we can size this properly
	zvals := make([][]byte, 0)
	for {
		if len(b) == 0 {
			return nil, nil, ErrUnterminated
		}
		if b[0] == rightbracket {
			if len(b) < 2 || b[1] != semicolon {
				return nil, nil, ErrUnterminated
			}
			container := Raw(zval.AppendContainerZvals(nil, zvals))
			fmt.Println("PARSE-CONT-RET", container.String(), string(b[2:]))
			return container, b[2:], nil
		}
		zv, rest, err := zsonParseField(b)
		if err != nil {
			return nil, nil, err
		}
		zvals = append(zvals, zv)
		b = rest
	}
}

// zsonParseField() parses the given bye array representing any value
// in the zson format.
func zsonParseField(b []byte) (Raw, []byte, error) {
	if b[0] == leftbracket {
		container, rest, err := zsonParseContainer(b)
		if err != nil {
			return nil, nil, err
		}
		fmt.Println("PARSE-FIELD RET CONTAINER", Raw(container).String())
		return container, rest, nil
	}
	i := 0
	for {
		if i >= len(b) {
			return nil, nil, ErrUnterminated
		}
		switch b[i] {
		case semicolon:
			zv := zval.AppendValue(nil, b[:i])
			fmt.Println("PARSE-FIELD RET FIELD", Raw(zv).String())
			return zv, b[i+1:], nil
		case backslash:
			// XXX need to implement full escape parsing,
			// for now just skip one character
			i += 1
		}
		i += 1
	}
}

func cString(vals [][]byte) string {
	s := ""
	for _, v := range vals {
		s += Raw(v).String()
	}
	return s
}

func (r Raw) String() string {
	s := ""
	for it := zval.Iter(r); !it.Done(); {
		v, container, err := it.Next()
		if err != nil {
			return err.Error()
		}
		if container {
			s += "(" + Raw(v).String() + ")"
		} else {
			s += "(" + string(v) + ")"
		}
	}
	return s
}
