package proc

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mccanne/zq/ast"
	"github.com/mccanne/zq/pkg/zeek"
	"github.com/mccanne/zq/pkg/zval"
)

var ErrNonAdjacent = errors.New("non adjacent fields")

type errNonAdjacent struct {
	record string
}

func (e errNonAdjacent) Error() string {
	return fmt.Sprintf("fields in record %s must be adjacent", e.record)
}

func (e errNonAdjacent) Unwrap() error {
	return ErrNonAdjacent
}

var ErrDuplicateFields = errors.New("duplicate fields")

type errDuplicateFields struct {
	field string
}

func (e errDuplicateFields) Error() string {
	return fmt.Sprintf("field %s is repeated", e.field)
}

func (e errDuplicateFields) Unwrap() error {
	return ErrDuplicateFields
}

// fieldInfo encodes the structure of a particular proc that writes a
// sequence of fields, which may potentially be inside nested records.
// This encoding enables the runtime processing to happen as efficiently
// as possible.  When handling an input record, we build an output record
// using a zval.Builder but when handling fields within nested records,
// calls to BeginContainer() and EndContainer() on the builder need to
// happen at the right times to yield the proper output structure.
// This is probably best illustrated with an example, consider the proc
// "cut a, b.c, b.d, x.y.z".
//
// At runtime, this needs to turn into the following actions:
// 1.  builder.Append([value of a from the input record])
// 2.  builder.BeginContainer()  // for "b"
// 3.  builder.Append([value of b.c from the input record])
// 4.  builder.Append([value of b.d from the input record])
// 5.  builder.EndContainer()    // for "b"
// 6.  builder.BeginContainer()  // for "x"
// 7.  builder.BeginContainer()  // for "x.y"
// 8.  builder.Append([value of x.y.z. from the input record])
// 9.  builder.EndContainer()    // for "x.y"
// 10. builder.EndContainer()    // for "y"
//
// This is encoded into the following fieldInfo objects:
//  {name: "a", fullname: "a", containerBegins: [], containerEnds: 0}         // step 1
//  {name: "c", fullname: "b.c", containerBegins: ["b"], containerEnds: 0}      // steps 2-3
//  {name: "d", fullname: "b.d", containerBegins: [], containerEnds: 1     }    // steps 4-5
//  {name: "z", fullname: "x.y.z", containerBegins: ["x", "y"], containerEnds: 2} // steps 6-10
type fieldInfo struct {
	name            string
	fullname        string
	containerBegins []string
	containerEnds   int
}

type ColumnBuilder struct {
	fields   []fieldInfo
	builder  *zval.Builder
	curField int
}

// Build the structures we need to construct output records efficiently.
// See the comment above for a description of the desired output.
// Note that we require any nested fields from the same parent record
// to be adjacent.  Alternatively we could re-order provided fields
// so the output record can be constructed efficiently, though we don't
// do this now since it might confuse users who expect to see output
// fields in the order they specified.
func NewColumnBuilder(exprs []ast.FieldExpr) (*ColumnBuilder, error) {
	seenRecords := make(map[string]bool)
	fieldInfos := make([]fieldInfo, 0, len(exprs))
	var currentRecord []string
	for i, field := range exprs {
		names, err := split(field)
		if err != nil {
			return nil, err
		}

		// Grab everything except the leaf field name and see if
		// it has changed from the previous field.  If it hasn't,
		// things are simple but if it has, we need to carefully
		// figure out which records we are stepping in and out of.
		record := names[:len(names)-1]
		var containerBegins []string
		if !sameRecord(record, currentRecord) {
			// currentRecord is what nested record the zval.Builder
			// is currently working on, record is the nested
			// record for the current field.  First figure out
			// what (if any) common parents are shared.
			l := len(currentRecord)
			if len(record) < l {
				l = len(record)
			}
			pos := 0
			for pos < l {
				if record[pos] != currentRecord[pos] {
					break
				}
				pos += 1
			}

			// Note any previously encoded records that are
			// now finished.
			if i > 0 {
				fieldInfos[i-1].containerEnds = len(currentRecord) - pos
			}

			// Validate any new records that we're starting
			// (i.e., ensure that we didn't handle fields from
			// the same record previously), then record the names
			// of all these records.
			for pos2 := pos; pos2 < len(record); pos2++ {
				recname := strings.Join(record[:pos2+1], ".")
				_, seen := seenRecords[recname]
				if seen {
					return nil, errNonAdjacent{recname}
				}
				seenRecords[recname] = true
				containerBegins = append(containerBegins, record[pos2])
			}
			currentRecord = record
		}
		fullname := strings.Join(names, ".")
		fname := names[len(names)-1]
		for _, fi := range fieldInfos {
			if fullname == fi.fullname {
				return nil, errDuplicateFields{fullname}
			}
		}
		fieldInfos = append(fieldInfos, fieldInfo{fname, fullname, containerBegins, 0})
	}
	if len(fieldInfos) > 0 {
		fieldInfos[len(fieldInfos)-1].containerEnds = len(currentRecord)
	}

	return &ColumnBuilder{
		fields:  fieldInfos,
		builder: zval.NewBuilder(),
	}, nil
}

// Split an ast.FieldExpr representing a chain of record field references
// into a list of strings representing the names.
// E.g., "x.y.z" -> ["x", "y", "z"]
func split(node ast.FieldExpr) ([]string, error) {
	switch n := node.(type) {
	case *ast.FieldRead:
		return []string{n.Field}, nil
	case *ast.FieldCall:
		if n.Fn != "RecordFieldRead" {
			return nil, fmt.Errorf("unexpected field op %s", n.Fn)
		}
		names, err := split(n.Field)
		if err != nil {
			return nil, err
		}
		return append(names, n.Param), nil
	default:
		return nil, fmt.Errorf("unexpected node type %T", node)
	}
}

func sameRecord(names1, names2 []string) bool {
	if len(names1) != len(names2) {
		return false
	}
	for i := range names1 {
		if names1[i] != names2[i] {
			return false
		}
	}
	return true
}

func (b *ColumnBuilder) Reset() {
	b.builder.Reset()
	b.curField = 0
}

func (b *ColumnBuilder) Append(leaf []byte, container bool) {
	field := b.fields[b.curField]
	b.curField++
	for range field.containerBegins {
		b.builder.BeginContainer()
	}
	b.builder.Append(leaf, container)
	for i := 0; i < field.containerEnds; i++ {
		b.builder.EndContainer()
	}
}

func (b *ColumnBuilder) Encode() (zval.Encoding, error) {
	if b.curField != len(b.fields) {
		return nil, errors.New("did not receive enough columns")
	}
	return b.builder.Encode(), nil
}

// A ColumnBuilder understands the shape of a sequence of FieldExprs
// (i.e., which columns are inside nested records) but not the types.
// TypedColumns takes an array of zeek.Types for the individual fields
// and constructs an array of zeek.Columns that reflects the fullly
// typed structure.  This is suitable for e.g. allocating a descriptor.
func (b *ColumnBuilder) TypedColumns(types []zeek.Type) []zeek.Column {
	type rec struct {
		name string
		cols []zeek.Column
	}
	current := &rec{"", nil}
	stack := make([]*rec, 1)
	stack[0] = current

	for i, field := range b.fields {
		for _, name := range field.containerBegins {
			current = &rec{name, nil}
			stack = append(stack, current)
		}

		current.cols = append(current.cols, zeek.Column{Name: field.name, Type: types[i]})

		for j := 0; j < field.containerEnds; j++ {
			recType := zeek.LookupTypeRecord(current.cols)
			slen := len(stack)
			stack = stack[:slen-1]
			cur := stack[slen-2]
			cur.cols = append(cur.cols, zeek.Column{Name: current.name, Type: recType})
			current = cur
		}
	}
	if len(stack) != 1 {
		panic("Mismatched container begin/end")
	}
	return stack[0].cols
}

func (c *ColumnBuilder) FullNames() []string {
	ret := make([]string, len(c.fields))
	for i, field := range c.fields {
		ret[i] = field.fullname
	}
	return ret
}
