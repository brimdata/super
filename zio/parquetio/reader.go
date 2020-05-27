package parquetio

import (
	"fmt"

	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zcode"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"

	"github.com/xitongsys/parquet-go/common"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/reader"
	"github.com/xitongsys/parquet-go/schema"
	"github.com/xitongsys/parquet-go/source"
)

type HandledType int

// These are all the types we can handle...
const (
	// un-annotated primitive types
	boolean = iota
	tint32
	tint64
	float
	double
	byteArray

	// XXX
	int96

	// annotated strings
	utf8
	enum
	json
	bson

	// annotated int64s
	timestampMilliseconds
	timestampMicroseconds
	timestampNanoseconds

	// XXX INTERVAL
	// XXX INT_*, UINT_* types

	// composite types
	list
)

func lookupPrimitiveType(typ *parquet.Type, cType *parquet.ConvertedType) (HandledType, error) {
	if cType != nil {
		switch *cType {
		case parquet.ConvertedType_UTF8:
			return utf8, nil
		case parquet.ConvertedType_JSON:
			return json, nil
		case parquet.ConvertedType_BSON:
			return bson, nil
		case parquet.ConvertedType_ENUM:
			return enum, nil
		case parquet.ConvertedType_TIMESTAMP_MILLIS:
			return timestampMilliseconds, nil
		case parquet.ConvertedType_TIMESTAMP_MICROS:
			return timestampMicroseconds, nil

		// XXX case parquet.ConvertedType_INTERVAL:

		default:
			return -1, fmt.Errorf("cannot convert Ctype %s", *cType)
		}

		// XXX handle logical types
	} else if typ != nil {
		switch *typ {
		case parquet.Type_BOOLEAN:
			return boolean, nil
		case parquet.Type_INT32:
			return tint32, nil
		case parquet.Type_INT64:
			return tint64, nil
		case parquet.Type_FLOAT:
			return float, nil
		case parquet.Type_DOUBLE:
			return double, nil
		case parquet.Type_BYTE_ARRAY:
			return byteArray, nil
		case parquet.Type_INT96:
			return int96, nil
		default:
			return -1, fmt.Errorf("cannot convert type %s", *typ)
		}
	} else {
		return -1, fmt.Errorf("cannot convert unknown type")
	}
}

func simpleParquetTypeToZngType(typ HandledType) zng.Type {
	switch typ {
	case boolean:
		return zng.TypeBool
	case tint32:
		return zng.TypeInt32
	case tint64:
		return zng.TypeInt64
	case float, double:
		return zng.TypeFloat64
	case byteArray:
		return zng.TypeBstring

	case utf8, enum, json:
		return zng.TypeString
	case bson:
		return zng.TypeBstring

	case timestampMilliseconds, timestampMicroseconds, timestampNanoseconds:
		return zng.TypeTime

	// XXX
	case int96:
		return zng.TypeInt64
	}

	// This is only reachable in the event of a programming error.
	panic(fmt.Sprintf("unhandled type %d", typ))
}

type Reader struct {
	pr      *reader.ParquetReader
	typ     *zng.TypeRecord
	columns []column
	record  int
	total   int
	builder *zcode.Builder
}

func NewReader(f source.ParquetFile, zctx *resolver.Context) (*Reader, error) {
	pr, err := reader.NewParquetReader(f, nil, 1)
	if err != nil {
		return nil, err
	}

	cols, err := buildColumns(pr)
	if err != nil {
		return nil, err
	}

	zcols := make([]zng.Column, len(cols))
	for i, c := range cols {
		zcols[i] = zng.Column{c.getName(), c.zngType(zctx)}
	}
	typ, err := zctx.LookupTypeRecord(zcols)
	if err != nil {
		return nil, err
	}

	return &Reader{
		pr:      pr,
		typ:     typ,
		columns: cols,
		total:   int(pr.GetNumRows()),
		builder: zcode.NewBuilder(),
	}, nil
}

// column abstracts away the handling of an indvidual column from a
// parquet file.  This interface currently has two concrete
// implementations, one for columns that just hold primitive values
// and one for columns that hold lists.
type column interface {
	zngType(zctx *resolver.Context) zng.Type
	append(builder *zcode.Builder) error
	getName() string
}

func buildColumns(pr *reader.ParquetReader) ([]column, error) {
	schema := pr.Footer.Schema

	// first element in the schema is the root, skip it.
	// for each reamaining column, build a column iterator
	// structure.
	var columns []column
	for i := 1; i < len(schema); {
		n := 1
		var col column
		var err error
		if schema[i].NumChildren != nil {
			n, col, err = newNestedColumn(schema, i, pr)
		} else {
			col, err = newSimpleColumn(*schema[i], pr)
		}
		i += n
		if err != nil {
			return nil, err
		}

		// XXX if no error but no type, just skip...
		if col == nil {
			continue
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// The parquet-go library converts column names into names that are valid
// public field names in a go structure.  This function recovers the
// original column name from the parquet schema.
// This is also complicated by the fact that a bunch of data structures
// inside parquet-go use fully-qualified names.  This functions takes the
// fully-qualified name as represented inside parquet-go
// (e.g., "Com46acme46zng_1234.P__path") and returns just the column name
// (i.e., "_path" in this example).
func translateColName(internalPath string, handler *schema.SchemaHandler) string {
	name := handler.InPathToExPath[internalPath]
	return name[len(handler.Infos[0].ExName)+1:]
}

func newSimpleColumn(el parquet.SchemaElement, pr *reader.ParquetReader) (column, error) {
	if el.RepetitionType != nil && *el.RepetitionType == parquet.FieldRepetitionType_REPEATED {
		return nil, fmt.Errorf("cannot convert repeated element %s", el.Name)
	}

	typ, err := lookupPrimitiveType(el.Type, el.ConvertedType)
	if err != nil {
		return nil, err
	}

	handler := pr.SchemaHandler
	path := []string{handler.Infos[0].InName, el.Name}
	pathStr := common.PathToStr(path)

	name := translateColName(pathStr, handler)

	maxRepetition, _ := handler.MaxRepetitionLevel(path)
	maxDefinition, _ := handler.MaxDefinitionLevel(path)

	iter := newColumnIterator(pr, el.Name, maxRepetition, maxDefinition)
	return &simpleColumn{
		name:          name,
		typ:           typ,
		iter:          iter,
		maxDefinition: maxDefinition,
	}, nil
}

// Given a schema element with child elements, recursively count its
// total number of descendents.  This is only used for skipping over
// columnns with an unrecognized structure.
func countChildren(els []*parquet.SchemaElement, i int) int {
	if i >= len(els) {
		return -1
	}
	if els[i].NumChildren == nil {
		return 1
	}

	n := int(*(els[i].NumChildren))
	j := i + 1
	for c := 0; c < n; c++ {
		cc := countChildren(els, j)
		if cc == -1 {
			return -1
		}
		j += cc
	}
	return j - i
}

func newNestedColumn(els []*parquet.SchemaElement, i int, pr *reader.ParquetReader) (int, column, error) {
	el := els[i]
	if el.ConvertedType != nil && *el.ConvertedType == parquet.ConvertedType_LIST {
		return newListColumn(els, i, pr)
	}
	if el.LogicalType != nil && el.LogicalType.LIST != nil {
		return newListColumn(els, i, pr)
	}

	// Skip this element and all its children...
	return countChildren(els, i), nil, nil
}

func newListColumn(els []*parquet.SchemaElement, i int, pr *reader.ParquetReader) (int, column, error) {
	// Per https://github.com/apache/parquet-format/blob/master/LogicalTypes.md#lists
	// List structure is:
	// <list-repetition> group <name> (LIST) {
	//   repeated group list {
	//     <element-repetition> <element-type> element;
	//   }
	// }
	//
	// First sanity check that we're looking at something with that
	// structure.

	el := els[i]
	if len(els) < i+2 {
		return 1, nil, fmt.Errorf("not enough nested elements for LIST %s", el.Name)
	}

	if el.RepetitionType == nil || *el.RepetitionType == parquet.FieldRepetitionType_REPEATED {
		return 1, nil, fmt.Errorf("list (field %s) must not be repeated", el.Name)
	}
	if el.NumChildren == nil || *el.NumChildren != 1 {
		return 1, nil, fmt.Errorf("LIST element (%s) should have 1 child", el.Name)
	}

	listEl := els[i+1]
	if listEl.RepetitionType == nil || *listEl.RepetitionType != parquet.FieldRepetitionType_REPEATED {
		return 1, nil, fmt.Errorf("list (field %s) must not be repeated", el.Name)
	}
	if listEl.NumChildren == nil || *listEl.NumChildren != 1 {
		return 1, nil, fmt.Errorf("LIST element (%s) should have 1 child", el.Name)
	}

	typeEl := els[i+2]
	typ, err := lookupPrimitiveType(typeEl.Type, typeEl.ConvertedType)
	if err != nil {
		return 3, nil, err
	}

	// This is something we can handle.  The column name correponds
	// to the outer element (el), but the actual values are kept in
	// the innermost nested element (typeEl).
	handler := pr.SchemaHandler
	path := []string{handler.Infos[0].InName, el.Name, listEl.Name, typeEl.Name}
	name := translateColName(common.PathToStr(path[:2]), handler)

	maxRepetition, _ := handler.MaxRepetitionLevel(path)
	maxDefinition, _ := handler.MaxDefinitionLevel(path)

	iter := newColumnIterator(pr, el.Name, maxRepetition, maxDefinition)

	c := listColumn{
		name:          name,
		innerType:     typ,
		iter:          iter,
		maxRepetition: maxRepetition,
		maxDefinition: maxDefinition,
	}

	return 3, &c, nil
}

// Read one primitive value from a column iterator and append it to the
// given zcode.Builder.  This is essentially the complete implemenntation
// of append() for a non-repeated column, and is used inside a loop for
// LIST-valued columns.
func appendItem(builder *zcode.Builder, typ HandledType, iter *columnIterator, maxDef int32) error {
	var dl int32
	switch typ {
	case boolean:
		var b bool
		b, _, dl = iter.nextBoolean()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			builder.AppendPrimitive(zng.EncodeBool(b))
		}
	case tint32:
		var i int32
		i, _, dl = iter.nextInt32()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			builder.AppendPrimitive(zng.EncodeInt(int64(i)))
		}
	case tint64:
		var i int64
		i, _, dl = iter.nextInt64()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			builder.AppendPrimitive(zng.EncodeInt(i))
		}
	case float:
		var f float64
		f, _, dl = iter.nextFloat()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			builder.AppendPrimitive(zng.EncodeFloat64(f))
		}
	case double:
		var f float64
		f, _, dl = iter.nextDouble()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			builder.AppendPrimitive(zng.EncodeFloat64(f))
		}
	case utf8, enum, json:
		var a []byte
		a, _, dl = iter.nextByteArray()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			builder.AppendPrimitive(zng.EncodeString(string(a)))
		}
	case byteArray, bson:
		var a []byte
		a, _, dl = iter.nextByteArray()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			builder.AppendPrimitive(zng.EncodeBstring(string(a)))
		}
	case timestampMilliseconds, timestampMicroseconds, timestampNanoseconds:
		var i int64
		i, _, dl = iter.nextInt64()
		if maxDef > dl {
			builder.AppendPrimitive(nil)
		} else {
			var ts nano.Ts
			switch typ {
			case timestampMilliseconds:
				ts = nano.Ts(i * 1000_000)
			case timestampMicroseconds:
				ts = nano.Ts(i * 1000)
			case timestampNanoseconds:
				ts = nano.Ts(i)
			}
			builder.AppendPrimitive(zng.EncodeTime(ts))
		}
	default:
		return fmt.Errorf("unhandled type %d", typ)
	}
	return nil
}

// simpleColumn handles a column from a parquet file that holds individual
// (non-repeated) primitive values.
type simpleColumn struct {
	name string
	typ  HandledType

	iter          *columnIterator
	maxDefinition int32
}

func (c *simpleColumn) getName() string { return c.name }

func (c *simpleColumn) zngType(zctx *resolver.Context) zng.Type {
	return simpleParquetTypeToZngType(c.typ)
}

// append reads the next value from this column and appends it to the
// given zcode.Builder.  This code represents an unwound and vastly
// simplified version of the code in the methods:
// parquet-go.reader.ParquetReader.read(), and
// parquet-go.marshal.Unmarshal()
func (c *simpleColumn) append(builder *zcode.Builder) error {
	return appendItem(builder, c.typ, c.iter, c.maxDefinition)
}

// listColumn handles a column from a parquet file that holds LIST
// structures as defined in the parquet spec.
type listColumn struct {
	name      string
	innerType HandledType

	iter          *columnIterator
	maxRepetition int32
	maxDefinition int32
}

func (c *listColumn) getName() string { return c.name }

func (c *listColumn) zngType(zctx *resolver.Context) zng.Type {
	inner := simpleParquetTypeToZngType(c.innerType)
	return zctx.LookupTypeArray(inner)
}

// append reads the next value from this column and appends it to the given
// zcode.Builder.  This code (together with the readNext() method represent
// an unwound and vastly simplified version of the code in the methods:
// parquet-go.reader.ParquetReader.read(), and
// parquet-go.marshal.Unmarshal()
func (c *listColumn) append(builder *zcode.Builder) error {
	dl := c.iter.peekDL()
	if c.maxDefinition > dl {
		builder.AppendContainer(nil)
		return nil
	}

	builder.BeginContainer()
	first := true
	for {
		rl := c.iter.peekRL()
		if first {
			first = false
		} else {
			if rl == 0 {
				break
			}
		}
		err := appendItem(builder, c.innerType, c.iter, c.maxDefinition)
		if err != nil {
			return err
		}
	}
	builder.EndContainer()
	return nil
}

func (r *Reader) Read() (*zng.Record, error) {
	if r.record == r.total {
		return nil, nil
	}
	r.record++

	r.builder.Reset()
	for _, c := range r.columns {
		if err := c.append(r.builder); err != nil {
			return nil, err
		}
	}
	return zng.NewRecord(r.typ, r.builder.Bytes())
}
