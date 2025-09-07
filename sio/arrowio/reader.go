package arrowio

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"unsafe"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/scode"
)

// Reader is a sio.Reader for the Arrow IPC stream format.
type Reader struct {
	sctx *super.Context
	rr   pqarrow.RecordReader

	typ              super.Type
	unionTagMappings map[string][]int

	rec arrow.Record
	i   int

	builder scode.Builder
	val     super.Value
}

func NewReader(sctx *super.Context, r io.Reader) (*Reader, error) {
	ipcReader, err := ipc.NewReader(r)
	if err != nil {
		return nil, err
	}
	ar, err := NewReaderFromRecordReader(sctx, ipcReader)
	if err != nil {
		ipcReader.Release()
		return nil, err
	}
	return ar, nil
}

func NewReaderFromRecordReader(sctx *super.Context, rr pqarrow.RecordReader) (*Reader, error) {
	r := &Reader{
		sctx:             sctx,
		rr:               rr,
		unionTagMappings: map[string][]int{},
	}
	typ, err := r.newType(arrow.StructOf(rr.Schema().Fields()...))
	if err != nil {
		return nil, err
	}
	r.typ = typ
	return r, nil
}

func UniquifyFieldNames(fields []super.Field) {
	names := map[string]int{}
	for i, f := range fields {
		if n := names[f.Name]; n > 0 {
			fields[i].Name += strconv.Itoa(n)
		}
		names[f.Name]++
	}
}

func (r *Reader) Close() error {
	if r.rr != nil {
		r.rr.Release()
		r.rr = nil
	}
	if r.rec != nil {
		r.rec.Release()
		r.rec = nil
	}
	return nil
}

func (r *Reader) Read() (*super.Value, error) {
	for r.rec == nil {
		rec, err := r.rr.Read()
		if err != nil {
			if err == io.EOF {
				return nil, nil
			}
			return nil, err
		}
		if rec.NumRows() > 0 {
			r.rec = rec
			r.i = 0
		} else {
			rec.Release()
		}
	}
	r.builder.Truncate()
	for _, array := range r.rec.Columns() {
		if err := r.buildScode(array, r.i); err != nil {
			return nil, err
		}
	}
	r.val = super.NewValue(r.typ, r.builder.Bytes())
	r.i++
	if r.i >= int(r.rec.NumRows()) {
		r.rec.Release()
		r.rec = nil
	}
	return &r.val, nil
}

var dayTimeIntervalFields = []super.Field{
	{Name: "days", Type: super.TypeInt32},
	{Name: "milliseconds", Type: super.TypeUint32},
}
var decimal128Fields = []super.Field{
	{Name: "high", Type: super.TypeInt64},
	{Name: "low", Type: super.TypeUint64},
}
var monthDayNanoIntervalFields = []super.Field{
	{Name: "month", Type: super.TypeInt32},
	{Name: "day", Type: super.TypeInt32},
	{Name: "nanoseconds", Type: super.TypeInt64},
}

func (r *Reader) newType(dt arrow.DataType) (super.Type, error) {
	// Order here follows that of the arrow.Time constants.
	switch dt.ID() {
	case arrow.NULL:
		return super.TypeNull, nil
	case arrow.BOOL:
		return super.TypeBool, nil
	case arrow.UINT8:
		return super.TypeUint8, nil
	case arrow.INT8:
		return super.TypeInt8, nil
	case arrow.UINT16:
		return super.TypeUint16, nil
	case arrow.INT16:
		return super.TypeInt16, nil
	case arrow.UINT32:
		return super.TypeUint32, nil
	case arrow.INT32:
		return super.TypeInt32, nil
	case arrow.UINT64:
		return super.TypeUint64, nil
	case arrow.INT64:
		return super.TypeInt64, nil
	case arrow.FLOAT16:
		return super.TypeFloat16, nil
	case arrow.FLOAT32:
		return super.TypeFloat32, nil
	case arrow.FLOAT64:
		return super.TypeFloat64, nil
	case arrow.STRING:
		return super.TypeString, nil
	case arrow.BINARY:
		return super.TypeBytes, nil
	case arrow.FIXED_SIZE_BINARY:
		width := strconv.Itoa(dt.(*arrow.FixedSizeBinaryType).ByteWidth)
		return r.sctx.LookupTypeNamed("arrow_fixed_size_binary_"+width, super.TypeBytes)
	case arrow.DATE32:
		return r.sctx.LookupTypeNamed("arrow_date32", super.TypeTime)
	case arrow.DATE64:
		return r.sctx.LookupTypeNamed("arrow_date64", super.TypeTime)
	case arrow.TIMESTAMP:
		if unit := dt.(*arrow.TimestampType).Unit; unit != arrow.Nanosecond {
			return r.sctx.LookupTypeNamed("arrow_timestamp_"+unit.String(), super.TypeTime)
		}
		return super.TypeTime, nil
	case arrow.TIME32:
		unit := dt.(*arrow.Time32Type).Unit.String()
		return r.sctx.LookupTypeNamed("arrow_time32_"+unit, super.TypeTime)
	case arrow.TIME64:
		unit := dt.(*arrow.Time64Type).Unit.String()
		return r.sctx.LookupTypeNamed("arrow_time64_"+unit, super.TypeTime)
	case arrow.INTERVAL_MONTHS:
		return r.sctx.LookupTypeNamed("arrow_month_interval", super.TypeInt32)
	case arrow.INTERVAL_DAY_TIME:
		typ, err := r.sctx.LookupTypeRecord(dayTimeIntervalFields)
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeNamed("arrow_day_time_interval", typ)
	case arrow.DECIMAL128:
		typ, err := r.sctx.LookupTypeRecord(decimal128Fields)
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeNamed("arrow_decimal128", typ)
	case arrow.DECIMAL256:
		return r.sctx.LookupTypeNamed("arrow_decimal256", r.sctx.LookupTypeArray(super.TypeUint64))
	case arrow.LIST:
		typ, err := r.newType(dt.(*arrow.ListType).Elem())
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeArray(typ), nil
	case arrow.STRUCT:
		var fields []super.Field
		for _, f := range dt.(*arrow.StructType).Fields() {
			typ, err := r.newType(f.Type)
			if err != nil {
				return nil, err
			}
			fields = append(fields, super.NewField(f.Name, typ))
		}
		UniquifyFieldNames(fields)
		return r.sctx.LookupTypeRecord(fields)
	case arrow.SPARSE_UNION, arrow.DENSE_UNION:
		return r.newUnionType(dt.(arrow.UnionType), dt.Fingerprint())
	case arrow.DICTIONARY:
		return r.newType(dt.(*arrow.DictionaryType).ValueType)
	case arrow.MAP:
		keyType, err := r.newType(dt.(*arrow.MapType).KeyType())
		if err != nil {
			return nil, err
		}
		itemType, err := r.newType(dt.(*arrow.MapType).ItemType())
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeMap(keyType, itemType), nil
	case arrow.FIXED_SIZE_LIST:
		typ, err := r.newType(dt.(*arrow.FixedSizeListType).Elem())
		if err != nil {
			return nil, err
		}
		size := strconv.Itoa(int(dt.(*arrow.FixedSizeListType).Len()))
		return r.sctx.LookupTypeNamed("arrow_fixed_size_list_"+size, r.sctx.LookupTypeArray(typ))
	case arrow.DURATION:
		if unit := dt.(*arrow.DurationType).Unit; unit != arrow.Nanosecond {
			return r.sctx.LookupTypeNamed("arrow_duration_"+unit.String(), super.TypeDuration)
		}
		return super.TypeDuration, nil
	case arrow.LARGE_STRING:
		return r.sctx.LookupTypeNamed("arrow_large_string", super.TypeString)
	case arrow.LARGE_BINARY:
		return r.sctx.LookupTypeNamed("arrow_large_binary", super.TypeBytes)
	case arrow.LARGE_LIST:
		typ, err := r.newType(dt.(*arrow.LargeListType).Elem())
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeNamed("arrow_large_list", r.sctx.LookupTypeArray(typ))
	case arrow.INTERVAL_MONTH_DAY_NANO:
		typ, err := r.sctx.LookupTypeRecord(monthDayNanoIntervalFields)
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeNamed("arrow_month_day_nano_interval", typ)
	default:
		return nil, fmt.Errorf("unimplemented Arrow type: %s", dt.Name())
	}
}

func (r *Reader) newUnionType(union arrow.UnionType, fingerprint string) (super.Type, error) {
	var types []super.Type
	for _, f := range union.Fields() {
		typ, err := r.newType(f.Type)
		if err != nil {
			return nil, err
		}
		types = append(types, typ)
	}
	uniqueTypes := super.UniqueTypes(slices.Clone(types))
	var x []int
Loop:
	for _, typ2 := range types {
		for i, typ := range uniqueTypes {
			if typ == typ2 {
				x = append(x, i)
				continue Loop
			}
		}
	}
	r.unionTagMappings[fingerprint] = x
	return r.sctx.LookupTypeUnion(uniqueTypes), nil
}

func (r *Reader) buildScode(a arrow.Array, i int) error {
	b := &r.builder
	if a.IsNull(i) {
		b.Append(nil)
		return nil
	}
	data := a.Data()
	// XXX Calling array.New*Data once per value (rather than once
	// per arrow.Array) is slow.
	//
	// Order here follows that of the arrow.Time constants.
	switch a.DataType().ID() {
	case arrow.NULL:
		b.Append(nil)
	case arrow.BOOL:
		b.Append(super.EncodeBool(array.NewBooleanData(data).Value(i)))
	case arrow.UINT8:
		b.Append(super.EncodeUint(uint64(array.NewUint8Data(data).Value(i))))
	case arrow.INT8:
		b.Append(super.EncodeInt(int64(array.NewInt8Data(data).Value(i))))
	case arrow.UINT16:
		b.Append(super.EncodeUint(uint64(array.NewUint16Data(data).Value(i))))
	case arrow.INT16:
		b.Append(super.EncodeInt(int64(array.NewInt16Data(data).Value(i))))
	case arrow.UINT32:
		b.Append(super.EncodeUint(uint64(array.NewUint32Data(data).Value(i))))
	case arrow.INT32:
		b.Append(super.EncodeInt(int64(array.NewInt32Data(data).Value(i))))
	case arrow.UINT64:
		b.Append(super.EncodeUint(array.NewUint64Data(data).Value(i)))
	case arrow.INT64:
		b.Append(super.EncodeInt(array.NewInt64Data(data).Value(i)))
	case arrow.FLOAT16:
		b.Append(super.EncodeFloat16(array.NewFloat16Data(data).Value(i).Float32()))
	case arrow.FLOAT32:
		b.Append(super.EncodeFloat32(array.NewFloat32Data(data).Value(i)))
	case arrow.FLOAT64:
		b.Append(super.EncodeFloat64(array.NewFloat64Data(data).Value(i)))
	case arrow.STRING:
		appendString(b, array.NewStringData(data).Value(i))
	case arrow.BINARY:
		b.Append(super.EncodeBytes(array.NewBinaryData(data).Value(i)))
	case arrow.FIXED_SIZE_BINARY:
		b.Append(super.EncodeBytes(array.NewFixedSizeBinaryData(data).Value(i)))
	case arrow.DATE32:
		b.Append(super.EncodeTime(nano.TimeToTs(array.NewDate32Data(data).Value(i).ToTime())))
	case arrow.DATE64:
		b.Append(super.EncodeTime(nano.TimeToTs(array.NewDate64Data(data).Value(i).ToTime())))
	case arrow.TIMESTAMP:
		unit := a.DataType().(*arrow.TimestampType).Unit
		b.Append(super.EncodeTime(nano.TimeToTs(array.NewTimestampData(data).Value(i).ToTime(unit))))
	case arrow.TIME32:
		unit := a.DataType().(*arrow.Time32Type).Unit
		b.Append(super.EncodeTime(nano.TimeToTs(array.NewTime32Data(data).Value(i).ToTime(unit))))
	case arrow.TIME64:
		unit := a.DataType().(*arrow.Time64Type).Unit
		b.Append(super.EncodeTime(nano.TimeToTs(array.NewTime64Data(data).Value(i).ToTime(unit))))
	case arrow.INTERVAL_MONTHS:
		b.Append(super.EncodeInt(int64(array.NewMonthIntervalData(data).Value(i))))
	case arrow.INTERVAL_DAY_TIME:
		v := array.NewDayTimeIntervalData(data).Value(i)
		b.BeginContainer()
		b.Append(super.EncodeInt(int64(v.Days)))
		b.Append(super.EncodeInt(int64(v.Milliseconds)))
		b.EndContainer()
	case arrow.DECIMAL128:
		v := array.NewDecimal128Data(data).Value(i)
		b.BeginContainer()
		b.Append(super.EncodeInt(v.HighBits()))
		b.Append(super.EncodeUint(v.LowBits()))
		b.EndContainer()
	case arrow.DECIMAL256:
		b.BeginContainer()
		for _, u := range array.NewDecimal256Data(data).Value(i).Array() {
			b.Append(super.EncodeUint(u))
		}
		b.EndContainer()
	case arrow.LIST:
		v := array.NewListData(data)
		start, end := v.ValueOffsets(i)
		return r.buildScodeList(v.ListValues(), int(start), int(end))
	case arrow.STRUCT:
		v := array.NewStructData(data)
		b.BeginContainer()
		for j := range v.NumField() {
			if err := r.buildScode(v.Field(j), i); err != nil {
				return err
			}
		}
		b.EndContainer()
	case arrow.SPARSE_UNION:
		return r.buildScodeUnion(array.NewSparseUnionData(data), data.DataType(), i)
	case arrow.DENSE_UNION:
		return r.buildScodeUnion(array.NewDenseUnionData(data), data.DataType(), i)
	case arrow.DICTIONARY:
		v := array.NewDictionaryData(data)
		return r.buildScode(v.Dictionary(), v.GetValueIndex(i))
	case arrow.MAP:
		v := array.NewMapData(data)
		keys, items := v.Keys(), v.Items()
		b.BeginContainer()
		for j, end := v.ValueOffsets(i); j < end; j++ {
			if err := r.buildScode(keys, int(j)); err != nil {
				return err
			}
			if err := r.buildScode(items, int(j)); err != nil {
				return err
			}
		}
		b.TransformContainer(super.NormalizeMap)
		b.EndContainer()
	case arrow.FIXED_SIZE_LIST:
		v := array.NewFixedSizeListData(data)
		return r.buildScodeList(v.ListValues(), 0, v.Len())
	case arrow.DURATION:
		d := nano.Duration(array.NewDurationData(data).Value(i))
		switch a.DataType().(*arrow.DurationType).Unit {
		case arrow.Second:
			d *= nano.Second
		case arrow.Millisecond:
			d *= nano.Millisecond
		case arrow.Microsecond:
			d *= nano.Microsecond
		}
		b.Append(super.EncodeDuration(d))
	case arrow.LARGE_STRING:
		appendString(b, array.NewLargeStringData(data).Value(i))
	case arrow.LARGE_BINARY:
		b.Append(super.EncodeBytes(array.NewLargeBinaryData(data).Value(i)))
	case arrow.LARGE_LIST:
		v := array.NewLargeListData(data)
		start, end := v.ValueOffsets(i)
		return r.buildScodeList(v.ListValues(), int(start), int(end))
	case arrow.INTERVAL_MONTH_DAY_NANO:
		v := array.NewMonthDayNanoIntervalData(data).Value(i)
		b.BeginContainer()
		b.Append(super.EncodeInt(int64(v.Months)))
		b.Append(super.EncodeInt(int64(v.Days)))
		b.Append(super.EncodeInt(v.Nanoseconds))
		b.EndContainer()
	default:
		return fmt.Errorf("unimplemented Arrow type: %s", a.DataType().Name())
	}
	return nil
}

func (r *Reader) buildScodeList(a arrow.Array, start, end int) error {
	r.builder.BeginContainer()
	for i := start; i < end; i++ {
		if err := r.buildScode(a, i); err != nil {
			return err
		}
	}
	r.builder.EndContainer()
	return nil
}

func (r *Reader) buildScodeUnion(u array.Union, dt arrow.DataType, i int) error {
	childID := u.ChildID(i)
	if u, ok := u.(*array.DenseUnion); ok {
		i = int(u.ValueOffset(i))
	}
	b := &r.builder
	if field := u.Field(childID); field.IsNull(i) {
		b.Append(nil)
	} else {
		b.BeginContainer()
		b.Append(super.EncodeInt(int64(r.unionTagMappings[dt.Fingerprint()][childID])))
		if err := r.buildScode(field, i); err != nil {
			return err
		}
		b.EndContainer()
	}
	return nil
}

func appendString(b *scode.Builder, s string) {
	if s == "" {
		b.Append(super.EncodeString(s))
	} else {
		// Avoid a call to runtime.stringtoslicebyte.
		b.Append(*(*[]byte)(unsafe.Pointer(&s)))
	}
}
