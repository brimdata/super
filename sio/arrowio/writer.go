package arrowio

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/decimal256"
	"github.com/apache/arrow-go/v18/arrow/float16"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

var (
	ErrMultipleTypes   = errors.New("arrowio: encountered multiple types (consider 'fuse')")
	ErrNotRecord       = errors.New("arrowio: not a record")
	ErrUnsupportedType = errors.New("arrowio: unsupported type")
)

// Writer is a sio.Writer for the Arrow IPC stream format.  Given values
// with appropriately named types (see the newArrowDataType implementation), it
// can write all Arrow types except dictionaries and sparse unions.  (Although
// dictionaries are not part of the super-structured data model, write support could be added
// using a named type.)
type Writer struct {
	NewWriterFunc    func(io.Writer, *arrow.Schema) (WriteCloser, error)
	w                io.WriteCloser
	writer           WriteCloser
	builder          *array.RecordBuilder
	unionTagMappings map[super.Type][]int
	typ              *super.TypeRecord
}

type WriteCloser interface {
	Write(rec arrow.Record) error
	Close() error
}

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		NewWriterFunc: func(w io.Writer, s *arrow.Schema) (WriteCloser, error) {
			return ipc.NewWriter(w, ipc.WithSchema(s)), nil
		},
		w:                w,
		unionTagMappings: map[super.Type][]int{},
	}
}

func (w *Writer) Close() error {
	var err error
	if w.writer != nil {
		err = w.flush(1)
		w.builder.Release()
		if err2 := w.writer.Close(); err == nil {
			err = err2
		}
		w.writer = nil
	}
	if err2 := w.w.Close(); err == nil {
		err = err2
	}
	return err
}

const recordBatchSize = 1024

func (w *Writer) Write(val super.Value) error {
	recType, ok := super.TypeUnder(val.Type()).(*super.TypeRecord)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotRecord, sup.FormatValue(val))
	}
	if w.typ == nil {
		w.typ = recType
		dt, err := w.newArrowDataType(recType)
		if err != nil {
			return err
		}
		schema := arrow.NewSchema(dt.(*arrow.StructType).Fields(), nil)
		w.builder = array.NewRecordBuilder(memory.DefaultAllocator, schema)
		w.builder.Reserve(recordBatchSize)
		w.writer, err = w.NewWriterFunc(w.w, schema)
		if err != nil {
			return err
		}
	} else if w.typ != recType {
		return fmt.Errorf("%w: %s and %s", ErrMultipleTypes, sup.FormatType(w.typ), sup.FormatType(recType))
	}
	it := scode.NewRecordIter(val.Bytes(), recType.Opts)
	for i, builder := range w.builder.Fields() {
		// We don't check for none since b will be nil and none is the
		b, none := it.Next(recType.Fields[i].Opt)
		w.buildArrowValue(builder, recType.Fields[i].Type, b, none)
	}
	return w.flush(recordBatchSize)
}

func (w *Writer) flush(min int) error {
	if w.builder.Field(0).Len() < min {
		return nil
	}
	rec := w.builder.NewRecord()
	defer rec.Release()
	w.builder.Reserve(recordBatchSize)
	return w.writer.Write(rec)
}

func (w *Writer) newArrowDataType(typ super.Type) (arrow.DataType, error) {
	var name string
	if n, ok := typ.(*super.TypeNamed); ok {
		name = n.Name
		typ = super.TypeUnder(n.Type)
	}
	// Order here follows that of the super.ID* and super.TypeValue* constants.
	switch typ := typ.(type) {
	case *super.TypeOfUint8:
		return arrow.PrimitiveTypes.Uint8, nil
	case *super.TypeOfUint16:
		return arrow.PrimitiveTypes.Uint16, nil
	case *super.TypeOfUint32:
		return arrow.PrimitiveTypes.Uint32, nil
	case *super.TypeOfUint64:
		return arrow.PrimitiveTypes.Uint64, nil
	case *super.TypeOfInt8:
		return arrow.PrimitiveTypes.Int8, nil
	case *super.TypeOfInt16:
		return arrow.PrimitiveTypes.Int16, nil
	case *super.TypeOfInt32:
		if name == "arrow_month_interval" {
			return arrow.FixedWidthTypes.MonthInterval, nil
		}
		return arrow.PrimitiveTypes.Int32, nil
	case *super.TypeOfInt64:
		return arrow.PrimitiveTypes.Int64, nil
	case *super.TypeOfDuration:
		switch name {
		case "arrow_duration_s":
			return arrow.FixedWidthTypes.Duration_s, nil
		case "arrow_duration_ms":
			return arrow.FixedWidthTypes.Duration_ms, nil
		case "arrow_duration_us":
			return arrow.FixedWidthTypes.Duration_us, nil
		case "arrow_day_time_interval":
			return arrow.FixedWidthTypes.DayTimeInterval, nil
		}
		return arrow.FixedWidthTypes.Duration_ns, nil
	case *super.TypeOfTime:
		switch name {
		case "arrow_date32":
			return arrow.FixedWidthTypes.Date32, nil
		case "arrow_date64":
			return arrow.FixedWidthTypes.Date64, nil
		case "arrow_timestamp_s":
			return arrow.FixedWidthTypes.Timestamp_s, nil
		case "arrow_timestamp_ms":
			return arrow.FixedWidthTypes.Timestamp_ms, nil
		case "arrow_timestamp_us":
			return arrow.FixedWidthTypes.Timestamp_us, nil
		case "arrow_time32_s":
			return arrow.FixedWidthTypes.Time32s, nil
		case "arrow_time32_ms":
			return arrow.FixedWidthTypes.Time32ms, nil
		case "arrow_time64_us":
			return arrow.FixedWidthTypes.Time64us, nil
		case "arrow_time64_ns":
			return arrow.FixedWidthTypes.Time64ns, nil
		}
		return arrow.FixedWidthTypes.Timestamp_ns, nil
	case *super.TypeOfFloat16:
		return arrow.FixedWidthTypes.Float16, nil
	case *super.TypeOfFloat32:
		return arrow.PrimitiveTypes.Float32, nil
	case *super.TypeOfFloat64:
		return arrow.PrimitiveTypes.Float64, nil
	case *super.TypeOfBool:
		return arrow.FixedWidthTypes.Boolean, nil
	case *super.TypeOfBytes:
		const prefix = "arrow_fixed_size_binary_"
		switch {
		case strings.HasPrefix(name, prefix):
			if width, err := strconv.Atoi(strings.TrimPrefix(name, prefix)); err == nil {
				return &arrow.FixedSizeBinaryType{ByteWidth: width}, nil
			}
		case name == "arrow_large_binary":
			return arrow.BinaryTypes.LargeBinary, nil
		}
		return arrow.BinaryTypes.Binary, nil
	case *super.TypeOfString:
		if name == "arrow_large_string" {
			return arrow.BinaryTypes.LargeString, nil
		}
		return arrow.BinaryTypes.String, nil
	case *super.TypeOfIP, *super.TypeOfNet, *super.TypeOfType:
		return arrow.BinaryTypes.String, nil
	case *super.TypeOfNull:
		return arrow.Null, nil
	case *super.TypeRecord:
		if len(typ.Fields) == 0 {
			return nil, fmt.Errorf("%w: empty record", ErrUnsupportedType)
		}
		switch name {
		case "arrow_day_time_interval":
			if typ.Opts != 0 {
				return nil, fmt.Errorf("%w: arrow_day_time_interval cannot have optional fields", ErrUnsupportedType)
			}
			if slices.Equal(typ.Fields, dayTimeIntervalFields) {
				return arrow.FixedWidthTypes.DayTimeInterval, nil
			}
		case "arrow_decimal128":
			if typ.Opts != 0 {
				return nil, fmt.Errorf("%w: arrow_decimal128 cannot have optional fields", ErrUnsupportedType)
			}
			if slices.Equal(typ.Fields, decimal128Fields) {
				return &arrow.Decimal128Type{}, nil
			}
		case "arrow_month_day_nano_interval":
			if typ.Opts != 0 {
				return nil, fmt.Errorf("%w: arrow_month_day_nano_interval cannot have optional fields", ErrUnsupportedType)
			}
			if slices.Equal(typ.Fields, monthDayNanoIntervalFields) {
				return arrow.FixedWidthTypes.MonthDayNanoInterval, nil
			}
		}
		var fields []arrow.Field
		for _, f := range typ.Fields {
			field, err := w.newArrowField(f.Name, f.Type)
			if err != nil {
				return nil, err
			}
			fields = append(fields, field)
		}
		return arrow.StructOf(fields...), nil
	case *super.TypeArray, *super.TypeSet:
		innerType := super.InnerType(typ)
		if name == "arrow_decimal256" && innerType == super.TypeUint64 {
			return &arrow.Decimal256Type{}, nil
		}
		field, err := w.newArrowField("", innerType)
		if err != nil {
			return nil, err
		}
		if s, ok := strings.CutPrefix(name, "arrow_fixed_size_list_"); ok {
			if n, err := strconv.Atoi(s); err == nil {
				return arrow.FixedSizeListOfField(int32(n), field), nil
			}
		}
		if name == "arrow_large_list" {
			return arrow.LargeListOfField(field), nil
		}
		return arrow.ListOfField(field), nil
	case *super.TypeMap:
		// Don't use newArrowField since Arrow map keys cannot be nullable.
		keyDT, err := w.newArrowDataType(typ.KeyType)
		if err != nil {
			return nil, err
		}
		keyField := arrow.Field{Type: keyDT}
		itemField, err := w.newArrowField("", typ.ValType)
		if err != nil {
			return nil, err
		}
		return arrow.MapOfFields(keyField, itemField), nil
	case *super.TypeUnion:
		if len(typ.Types) > math.MaxUint8 {
			return nil, fmt.Errorf("%w: union with more than %d fields", ErrUnsupportedType, math.MaxUint8)
		}
		var fields []arrow.Field
		var typeCodes []arrow.UnionTypeCode
		var mapping []int
		for _, typ := range typ.Types {
			f, err := w.newArrowField("", typ)
			if err != nil {
				return nil, err
			}
			if j := slices.IndexFunc(fields, func(ff arrow.Field) bool { return ff.Equal(f) }); j > -1 {
				mapping = append(mapping, j)
				continue
			}
			fields = append(fields, f)
			typeCode := len(typeCodes)
			typeCodes = append(typeCodes, arrow.UnionTypeCode(typeCode))
			mapping = append(mapping, typeCode)
		}
		w.unionTagMappings[typ] = mapping
		return arrow.DenseUnionOf(fields, typeCodes), nil
	case *super.TypeEnum, *super.TypeError:
		return arrow.BinaryTypes.String, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, sup.FormatType(typ))
	}
}

func (w *Writer) newArrowField(name string, typ super.Type) (arrow.Field, error) {
	var nullable bool
	if u, ok := nullableUnion(typ); ok {
		nullable = true
		if u.Types[0] != super.TypeNull {
			typ = u.Types[0]
		} else {
			typ = u.Types[1]
		}
	} else {
		// pqarrow requires that an arrow.NULL field be nullable.
		nullable = typ == super.TypeNull
	}
	dt, err := w.newArrowDataType(typ)
	if err != nil {
		return arrow.Field{}, err
	}
	return arrow.Field{Name: name, Type: dt, Nullable: nullable}, nil
}

func (w *Writer) buildArrowValue(b array.Builder, typ super.Type, bytes scode.Bytes, none bool) {
	if none {
		// This is a None from an optional field.
		b.AppendNull()
		return
	}
	if u, ok := nullableUnion(typ); ok {
		typ, bytes = u.Untag(bytes)
		if typ == super.TypeNull {
			b.AppendNull()
			return
		}
	}
	var name string
	if n, ok := typ.(*super.TypeNamed); ok {
		name = n.Name
		typ = super.TypeUnder(n.Type)
	}
	// Order here follows that of the arrow.Type constants.
	switch b := b.(type) {
	case *array.NullBuilder:
		b.AppendNull()
	case *array.BooleanBuilder:
		b.Append(super.DecodeBool(bytes))
	case *array.Uint8Builder:
		b.Append(uint8(super.DecodeUint(bytes)))
	case *array.Int8Builder:
		b.Append(int8(super.DecodeInt(bytes)))
	case *array.Uint16Builder:
		b.Append(uint16(super.DecodeUint(bytes)))
	case *array.Int16Builder:
		b.Append(int16(super.DecodeInt(bytes)))
	case *array.Uint32Builder:
		b.Append(uint32(super.DecodeUint(bytes)))
	case *array.Int32Builder:
		b.Append(int32(super.DecodeInt(bytes)))
	case *array.Uint64Builder:
		b.Append(super.DecodeUint(bytes))
	case *array.Int64Builder:
		b.Append(super.DecodeInt(bytes))
	case *array.Float16Builder:
		b.Append(float16.New(super.DecodeFloat16(bytes)))
	case *array.Float32Builder:
		b.Append(super.DecodeFloat32(bytes))
	case *array.Float64Builder:
		b.Append(super.DecodeFloat64(bytes))
	case *array.StringBuilder:
		switch typ := typ.(type) {
		case *super.TypeOfString:
			b.Append(super.DecodeString(bytes))
		case *super.TypeOfIP:
			b.Append(super.DecodeIP(bytes).String())
		case *super.TypeOfNet:
			b.Append(super.DecodeNet(bytes).String())
		case *super.TypeOfType:
			b.Append(sup.FormatTypeValue(bytes))
		case *super.TypeEnum:
			s, err := typ.Symbol(int(super.DecodeUint(bytes)))
			if err != nil {
				panic(fmt.Sprintf("decoding %s with bytes %s: %s", sup.FormatType(typ), hex.EncodeToString(bytes), err))
			}
			b.Append(s)
		case *super.TypeError:
			b.Append(sup.FormatValue(super.NewValue(typ, bytes)))
		default:
			panic(fmt.Sprintf("unexpected type for StringBuilder: %s", sup.FormatType(typ)))
		}
	case *array.BinaryBuilder:
		b.Append(super.DecodeBytes(bytes))
	case *array.FixedSizeBinaryBuilder:
		b.Append(super.DecodeBytes(bytes))
	case *array.Date32Builder:
		b.Append(arrow.Date32FromTime(super.DecodeTime(bytes).Time()))
	case *array.Date64Builder:
		b.Append(arrow.Date64FromTime(super.DecodeTime(bytes).Time()))
	case *array.TimestampBuilder:
		ts := super.DecodeTime(bytes)
		switch name {
		case "arrow_timestamp_s":
			ts /= nano.Ts(nano.Second)
		case "arrow_timestamp_ms":
			ts /= nano.Ts(nano.Millisecond)
		case "arrow_timestamp_us":
			ts /= nano.Ts(nano.Microsecond)
		}
		b.Append(arrow.Timestamp(ts))
	case *array.Time32Builder:
		ts := super.DecodeTime(bytes)
		switch name {
		case "arrow_time32_s":
			ts /= nano.Ts(nano.Second)
		case "arrow_time32_ms":
			ts /= nano.Ts(nano.Millisecond)
		default:
			panic(fmt.Sprintf("unexpected type name for Time32Builder: %s", sup.FormatType(typ)))
		}
		b.Append(arrow.Time32(ts))
	case *array.Time64Builder:
		ts := super.DecodeTime(bytes)
		if name == "arrow_time64_us" {
			ts /= nano.Ts(nano.Microsecond)
		}
		b.Append(arrow.Time64(ts))
	case *array.MonthIntervalBuilder:
		b.Append(arrow.MonthInterval(super.DecodeInt(bytes)))
	case *array.DayTimeIntervalBuilder:
		it := scode.NewRecordIter(bytes, 0)
		days, _ := it.Next(false)
		ms, _ := it.Next(false)
		b.Append(arrow.DayTimeInterval{
			Days:         int32(super.DecodeInt(days)),
			Milliseconds: int32(super.DecodeInt(ms)),
		})
	case *array.Decimal128Builder:
		it := scode.NewRecordIter(bytes, 0)
		high, _ := it.Next(false)
		low, _ := it.Next(false)
		b.Append(decimal128.New(super.DecodeInt(high), super.DecodeUint(low)))
	case *array.Decimal256Builder:
		it := bytes.Iter()
		x4 := super.DecodeUint(it.Next())
		x3 := super.DecodeUint(it.Next())
		x2 := super.DecodeUint(it.Next())
		x1 := super.DecodeUint(it.Next())
		b.Append(decimal256.New(x1, x2, x3, x4))
	case *array.ListBuilder:
		w.buildArrowListValue(b, typ, bytes)
	case *array.StructBuilder:
		b.Append(true)
		recType := super.TypeRecordOf(typ)
		it := scode.NewRecordIter(bytes, recType.Opts)
		for i, field := range recType.Fields {
			elem, none := it.Next(field.Opt)
			w.buildArrowValue(b.FieldBuilder(i), field.Type, elem, none)
		}
	case *array.DenseUnionBuilder:
		it := bytes.Iter()
		tag := super.DecodeInt(it.Next())
		typeCode := w.unionTagMappings[typ][tag]
		b.Append(arrow.UnionTypeCode(typeCode))
		w.buildArrowValue(b.Child(typeCode), typ.(*super.TypeUnion).Types[tag], it.Next(), false)
	case *array.MapBuilder:
		b.Append(true)
		typ := super.TypeUnder(typ).(*super.TypeMap)
		for it := bytes.Iter(); !it.Done(); {
			w.buildArrowValue(b.KeyBuilder(), typ.KeyType, it.Next(), false)
			w.buildArrowValue(b.ItemBuilder(), typ.ValType, it.Next(), false)
		}
	case *array.FixedSizeListBuilder:
		w.buildArrowListValue(b, typ, bytes)
	case *array.DurationBuilder:
		d := super.DecodeDuration(bytes)
		switch name {
		case "arrow_duration_s":
			d /= nano.Second
		case "arrow_duration_ms":
			d /= nano.Millisecond
		case "arrow_duration_us":
			d /= nano.Microsecond
		}
		b.Append(arrow.Duration(d))
	case *array.LargeStringBuilder:
		b.Append(super.DecodeString(bytes))
	case *array.LargeListBuilder:
		w.buildArrowListValue(b, typ, bytes)
	case *array.MonthDayNanoIntervalBuilder:
		it := scode.NewRecordIter(bytes, 0)
		months, _ := it.Next(false)
		days, _ := it.Next(false)
		nanos, _ := it.Next(false)
		b.Append(arrow.MonthDayNanoInterval{
			Months:      int32(super.DecodeInt(months)),
			Days:        int32(super.DecodeInt(days)),
			Nanoseconds: super.DecodeInt(nanos),
		})
	default:
		panic(fmt.Sprintf("unknown builder type %T", b))
	}
}

func (w *Writer) buildArrowListValue(b array.ListLikeBuilder, typ super.Type, bytes scode.Bytes) {
	b.Append(true)
	for it := bytes.Iter(); !it.Done(); {
		w.buildArrowValue(b.ValueBuilder(), super.InnerType(typ), it.Next(), false)
	}
}

func nullableUnion(typ super.Type) (*super.TypeUnion, bool) {
	u, ok := typ.(*super.TypeUnion)
	return u, ok && len(u.Types) == 2 && (u.Types[0] == super.TypeNull || u.Types[1] == super.TypeNull)
}
