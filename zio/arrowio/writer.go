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

	"github.com/apache/arrow/go/v14/arrow"
	"github.com/apache/arrow/go/v14/arrow/array"
	"github.com/apache/arrow/go/v14/arrow/decimal128"
	"github.com/apache/arrow/go/v14/arrow/decimal256"
	"github.com/apache/arrow/go/v14/arrow/float16"
	"github.com/apache/arrow/go/v14/arrow/ipc"
	"github.com/apache/arrow/go/v14/arrow/memory"
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zson"
)

var (
	ErrMultipleTypes   = errors.New("arrowio: encountered multiple types (consider 'fuse')")
	ErrNotRecord       = errors.New("arrowio: not a record")
	ErrUnsupportedType = errors.New("arrowio: unsupported type")
)

// Writer is a zio.Writer for the Arrow IPC stream format.  Given Zed values
// with appropriately named types (see the newArrowDataType implementation), it
// can write all Arrow types except dictionaries and sparse unions.  (Although
// dictionaries are not part of the Zed data model, write support could be added
// using a named type.)
type Writer struct {
	NewWriterFunc    func(io.Writer, *arrow.Schema) (WriteCloser, error)
	w                io.WriteCloser
	writer           WriteCloser
	builder          *array.RecordBuilder
	unionTagMappings map[zed.Type][]int
	typ              *zed.TypeRecord
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
		unionTagMappings: map[zed.Type][]int{},
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

func (w *Writer) Write(val zed.Value) error {
	recType, ok := zed.TypeUnder(val.Type()).(*zed.TypeRecord)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotRecord, zson.FormatValue(val))
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
		return fmt.Errorf("%w: %s and %s", ErrMultipleTypes, zson.FormatType(w.typ), zson.FormatType(recType))
	}
	it := val.Bytes().Iter()
	for i, builder := range w.builder.Fields() {
		var b zcode.Bytes
		if it != nil {
			b = it.Next()
		}
		w.buildArrowValue(builder, recType.Fields[i].Type, b)
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

func (w *Writer) newArrowDataType(typ zed.Type) (arrow.DataType, error) {
	var name string
	if n, ok := typ.(*zed.TypeNamed); ok {
		name = n.Name
		typ = zed.TypeUnder(n.Type)
	}
	// Order here follows that of the zed.ID* and zed.TypeValue* constants.
	switch typ := typ.(type) {
	case *zed.TypeOfUint8:
		return arrow.PrimitiveTypes.Uint8, nil
	case *zed.TypeOfUint16:
		return arrow.PrimitiveTypes.Uint16, nil
	case *zed.TypeOfUint32:
		return arrow.PrimitiveTypes.Uint32, nil
	case *zed.TypeOfUint64:
		return arrow.PrimitiveTypes.Uint64, nil
	case *zed.TypeOfInt8:
		return arrow.PrimitiveTypes.Int8, nil
	case *zed.TypeOfInt16:
		return arrow.PrimitiveTypes.Int16, nil
	case *zed.TypeOfInt32:
		if name == "arrow_month_interval" {
			return arrow.FixedWidthTypes.MonthInterval, nil
		}
		return arrow.PrimitiveTypes.Int32, nil
	case *zed.TypeOfInt64:
		return arrow.PrimitiveTypes.Int64, nil
	case *zed.TypeOfDuration:
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
	case *zed.TypeOfTime:
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
	case *zed.TypeOfFloat16:
		return arrow.FixedWidthTypes.Float16, nil
	case *zed.TypeOfFloat32:
		return arrow.PrimitiveTypes.Float32, nil
	case *zed.TypeOfFloat64:
		return arrow.PrimitiveTypes.Float64, nil
	case *zed.TypeOfBool:
		return arrow.FixedWidthTypes.Boolean, nil
	case *zed.TypeOfBytes:
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
	case *zed.TypeOfString:
		if name == "arrow_large_string" {
			return arrow.BinaryTypes.LargeString, nil
		}
		return arrow.BinaryTypes.String, nil
	case *zed.TypeOfIP, *zed.TypeOfNet, *zed.TypeOfType:
		return arrow.BinaryTypes.String, nil
	case *zed.TypeOfNull:
		return arrow.Null, nil
	case *zed.TypeRecord:
		if len(typ.Fields) == 0 {
			return nil, fmt.Errorf("%w: empty record", ErrUnsupportedType)
		}
		switch name {
		case "arrow_day_time_interval":
			if slices.Equal(typ.Fields, dayTimeIntervalFields) {
				return arrow.FixedWidthTypes.DayTimeInterval, nil
			}
		case "arrow_decimal128":
			if slices.Equal(typ.Fields, decimal128Fields) {
				return &arrow.Decimal128Type{}, nil
			}
		case "arrow_month_day_nano_interval":
			if slices.Equal(typ.Fields, monthDayNanoIntervalFields) {
				return arrow.FixedWidthTypes.MonthDayNanoInterval, nil
			}
		}
		var fields []arrow.Field
		for _, field := range typ.Fields {
			dt, err := w.newArrowDataType(field.Type)
			if err != nil {
				return nil, err
			}
			fields = append(fields, arrow.Field{
				Name:     field.Name,
				Type:     dt,
				Nullable: true,
			})
		}
		return arrow.StructOf(fields...), nil
	case *zed.TypeArray, *zed.TypeSet:
		dt, err := w.newArrowDataType(zed.InnerType(typ))
		if err != nil {
			return nil, err
		}
		const prefix = "arrow_fixed_size_list_"
		switch {
		case strings.HasPrefix(name, prefix):
			if n, err := strconv.Atoi(strings.TrimPrefix(name, prefix)); err == nil {
				return arrow.FixedSizeListOf(int32(n), dt), nil
			}
		case name == "arrow_decimal256":
			if inner := zed.InnerType(typ); inner == zed.TypeUint64 {
				return &arrow.Decimal256Type{}, nil
			}
		case name == "arrow_large_list":
			return arrow.LargeListOf(dt), nil
		}
		return arrow.ListOf(dt), nil
	case *zed.TypeMap:
		keyDT, err := w.newArrowDataType(typ.KeyType)
		if err != nil {
			return nil, err
		}
		valDT, err := w.newArrowDataType(typ.ValType)
		if err != nil {
			return nil, err
		}
		return arrow.MapOf(keyDT, valDT), nil
	case *zed.TypeUnion:
		if len(typ.Types) > math.MaxUint8 {
			return nil, fmt.Errorf("%w: union with more than %d fields", ErrUnsupportedType, math.MaxUint8)
		}
		var fields []arrow.Field
		var typeCodes []arrow.UnionTypeCode
		var mapping []int
		for _, typ := range typ.Types {
			dt, err := w.newArrowDataType(typ)
			if err != nil {
				return nil, err
			}
			if j := slices.IndexFunc(fields, func(f arrow.Field) bool { return arrow.TypeEqual(f.Type, dt) }); j > -1 {
				mapping = append(mapping, j)
				continue
			}
			fields = append(fields, arrow.Field{
				Type:     dt,
				Nullable: true,
			})
			typeCode := len(typeCodes)
			typeCodes = append(typeCodes, arrow.UnionTypeCode(typeCode))
			mapping = append(mapping, typeCode)
		}
		w.unionTagMappings[typ] = mapping
		return arrow.DenseUnionOf(fields, typeCodes), nil
	case *zed.TypeEnum, *zed.TypeError:
		return arrow.BinaryTypes.String, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, zson.FormatType(typ))
	}
}

func (w *Writer) buildArrowValue(b array.Builder, typ zed.Type, bytes zcode.Bytes) {
	if bytes == nil {
		b.AppendNull()
		return
	}
	var name string
	if n, ok := typ.(*zed.TypeNamed); ok {
		name = n.Name
		typ = zed.TypeUnder(n.Type)
	}
	// Order here follows that of the arrow.Type constants.
	switch b := b.(type) {
	case *array.NullBuilder:
		b.AppendNull()
	case *array.BooleanBuilder:
		b.Append(zed.DecodeBool(bytes))
	case *array.Uint8Builder:
		b.Append(uint8(zed.DecodeUint(bytes)))
	case *array.Int8Builder:
		b.Append(int8(zed.DecodeInt(bytes)))
	case *array.Uint16Builder:
		b.Append(uint16(zed.DecodeUint(bytes)))
	case *array.Int16Builder:
		b.Append(int16(zed.DecodeInt(bytes)))
	case *array.Uint32Builder:
		b.Append(uint32(zed.DecodeUint(bytes)))
	case *array.Int32Builder:
		b.Append(int32(zed.DecodeInt(bytes)))
	case *array.Uint64Builder:
		b.Append(zed.DecodeUint(bytes))
	case *array.Int64Builder:
		b.Append(zed.DecodeInt(bytes))
	case *array.Float16Builder:
		b.Append(float16.New(zed.DecodeFloat16(bytes)))
	case *array.Float32Builder:
		b.Append(zed.DecodeFloat32(bytes))
	case *array.Float64Builder:
		b.Append(zed.DecodeFloat64(bytes))
	case *array.StringBuilder:
		switch typ := typ.(type) {
		case *zed.TypeOfString:
			b.Append(zed.DecodeString(bytes))
		case *zed.TypeOfIP:
			b.Append(zed.DecodeIP(bytes).String())
		case *zed.TypeOfNet:
			b.Append(zed.DecodeNet(bytes).String())
		case *zed.TypeOfType:
			b.Append(zson.FormatTypeValue(bytes))
		case *zed.TypeEnum:
			s, err := typ.Symbol(int(zed.DecodeUint(bytes)))
			if err != nil {
				panic(fmt.Sprintf("decoding %s with bytes %s: %s", zson.FormatType(typ), hex.EncodeToString(bytes), err))
			}
			b.Append(s)
		case *zed.TypeError:
			b.Append(zson.FormatTypeAndBytes(typ, bytes))
		default:
			panic(fmt.Sprintf("unexpected Zed type for StringBuilder: %s", zson.FormatType(typ)))
		}
	case *array.BinaryBuilder:
		b.Append(zed.DecodeBytes(bytes))
	case *array.FixedSizeBinaryBuilder:
		b.Append(zed.DecodeBytes(bytes))
	case *array.Date32Builder:
		b.Append(arrow.Date32FromTime(zed.DecodeTime(bytes).Time()))
	case *array.Date64Builder:
		b.Append(arrow.Date64FromTime(zed.DecodeTime(bytes).Time()))
	case *array.TimestampBuilder:
		ts := zed.DecodeTime(bytes)
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
		ts := zed.DecodeTime(bytes)
		switch name {
		case "arrow_time32_s":
			ts /= nano.Ts(nano.Second)
		case "arrow_time32_ms":
			ts /= nano.Ts(nano.Millisecond)
		default:
			panic(fmt.Sprintf("unexpected Zed type name for Time32Builder: %s", zson.FormatType(typ)))
		}
		b.Append(arrow.Time32(ts))
	case *array.Time64Builder:
		ts := zed.DecodeTime(bytes)
		if name == "arrow_time64_us" {
			ts /= nano.Ts(nano.Microsecond)
		}
		b.Append(arrow.Time64(ts))
	case *array.MonthIntervalBuilder:
		b.Append(arrow.MonthInterval(zed.DecodeInt(bytes)))
	case *array.DayTimeIntervalBuilder:
		it := bytes.Iter()
		b.Append(arrow.DayTimeInterval{
			Days:         int32(zed.DecodeInt(it.Next())),
			Milliseconds: int32(zed.DecodeInt(it.Next())),
		})
	case *array.Decimal128Builder:
		it := bytes.Iter()
		high := zed.DecodeInt(it.Next())
		low := zed.DecodeUint(it.Next())
		b.Append(decimal128.New(high, low))
	case *array.Decimal256Builder:
		it := bytes.Iter()
		x4 := zed.DecodeUint(it.Next())
		x3 := zed.DecodeUint(it.Next())
		x2 := zed.DecodeUint(it.Next())
		x1 := zed.DecodeUint(it.Next())
		b.Append(decimal256.New(x1, x2, x3, x4))
	case *array.ListBuilder:
		w.buildArrowListValue(b, typ, bytes)
	case *array.StructBuilder:
		b.Append(true)
		it := bytes.Iter()
		for i, field := range zed.TypeRecordOf(typ).Fields {
			w.buildArrowValue(b.FieldBuilder(i), field.Type, it.Next())
		}
	case *array.DenseUnionBuilder:
		it := bytes.Iter()
		tag := zed.DecodeInt(it.Next())
		typeCode := w.unionTagMappings[typ][tag]
		b.Append(arrow.UnionTypeCode(typeCode))
		w.buildArrowValue(b.Child(typeCode), typ.(*zed.TypeUnion).Types[tag], it.Next())
	case *array.MapBuilder:
		b.Append(true)
		typ := zed.TypeUnder(typ).(*zed.TypeMap)
		for it := bytes.Iter(); !it.Done(); {
			w.buildArrowValue(b.KeyBuilder(), typ.KeyType, it.Next())
			w.buildArrowValue(b.ItemBuilder(), typ.ValType, it.Next())
		}
	case *array.FixedSizeListBuilder:
		w.buildArrowListValue(b, typ, bytes)
	case *array.DurationBuilder:
		d := zed.DecodeDuration(bytes)
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
		b.Append(zed.DecodeString(bytes))
	case *array.LargeListBuilder:
		w.buildArrowListValue(b, typ, bytes)
	case *array.MonthDayNanoIntervalBuilder:
		it := bytes.Iter()
		b.Append(arrow.MonthDayNanoInterval{
			Months:      int32(zed.DecodeInt(it.Next())),
			Days:        int32(zed.DecodeInt(it.Next())),
			Nanoseconds: zed.DecodeInt(it.Next()),
		})
	default:
		panic(fmt.Sprintf("unknown builder type %T", b))
	}
}

func (w *Writer) buildArrowListValue(b array.ListLikeBuilder, typ zed.Type, bytes zcode.Bytes) {
	b.Append(true)
	for it := bytes.Iter(); !it.Done(); {
		w.buildArrowValue(b.ValueBuilder(), zed.InnerType(typ), it.Next())
	}
}
