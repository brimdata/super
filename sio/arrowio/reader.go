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
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type RecordBatchReader interface {
	Read() (arrow.RecordBatch, error)
	Release()
	Schema() *arrow.Schema
}

// Reader is a sio.Reader for the Arrow IPC stream format.
type Reader struct {
	sctx *super.Context
	rbr  RecordBatchReader

	topLevelFields   []arrow.Field
	topLevelType     *super.TypeRecord
	unionTagMappings map[*super.TypeUnion][]int

	batch arrow.RecordBatch
	i     int

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

func NewReaderFromRecordReader(sctx *super.Context, rbr RecordBatchReader) (*Reader, error) {
	fields := rbr.Schema().Fields()
	r := &Reader{
		sctx:             sctx,
		rbr:              rbr,
		topLevelFields:   fields,
		unionTagMappings: map[*super.TypeUnion][]int{},
	}
	typ, err := r.newTypeFromDataType(arrow.StructOf(fields...))
	if err != nil {
		return nil, err
	}
	r.topLevelType = typ.(*super.TypeRecord)
	return r, nil
}

func (r *Reader) Type() super.Type {
	return r.topLevelType
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
	if r.rbr != nil {
		r.rbr.Release()
		r.rbr = nil
	}
	if r.batch != nil {
		r.batch.Release()
		r.batch = nil
	}
	return nil
}

func (r *Reader) Read() (*super.Value, error) {
	for r.batch == nil {
		batch, err := r.rbr.Read()
		if err != nil {
			if err == io.EOF {
				return nil, nil
			}
			return nil, err
		}
		if batch.NumRows() > 0 {
			r.batch = batch
			r.i = 0
		} else {
			batch.Release()
		}
	}
	r.builder.Truncate()
	for j, array := range r.batch.Columns() {
		typ := r.topLevelType.Fields[j].Type
		nullable := r.topLevelFields[j].Nullable
		if err := r.buildScodeWithNullable(typ, array, r.i, nullable); err != nil {
			return nil, err
		}
	}
	r.val = super.NewValue(r.topLevelType, r.builder.Bytes())
	r.i++
	if r.i >= int(r.batch.NumRows()) {
		r.batch.Release()
		r.batch = nil
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

func (r *Reader) newTypeFromDataType(dt arrow.DataType) (super.Type, error) {
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
		typ, err := r.newTypeFromField(dt.(*arrow.ListType).ElemField())
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeArray(typ), nil
	case arrow.STRUCT:
		var fields []super.Field
		for _, f := range dt.(*arrow.StructType).Fields() {
			typ, err := r.newTypeFromField(f)
			if err != nil {
				return nil, err
			}
			fields = append(fields, super.NewField(f.Name, typ))
		}
		UniquifyFieldNames(fields)
		return r.sctx.LookupTypeRecord(fields)
	case arrow.SPARSE_UNION, arrow.DENSE_UNION:
		return r.newUnionType(dt.(arrow.UnionType))
	case arrow.DICTIONARY:
		return r.newTypeFromDataType(dt.(*arrow.DictionaryType).ValueType)
	case arrow.MAP:
		keyType, err := r.newTypeFromField(dt.(*arrow.MapType).KeyField())
		if err != nil {
			return nil, err
		}
		itemType, err := r.newTypeFromField(dt.(*arrow.MapType).ItemField())
		if err != nil {
			return nil, err
		}
		return r.sctx.LookupTypeMap(keyType, itemType), nil
	case arrow.FIXED_SIZE_LIST:
		typ, err := r.newTypeFromField(dt.(*arrow.FixedSizeListType).ElemField())
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
		typ, err := r.newTypeFromField(dt.(*arrow.LargeListType).ElemField())
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

func (r *Reader) newTypeFromField(f arrow.Field) (super.Type, error) {
	typ, err := r.newTypeFromDataType(f.Type)
	if err != nil {
		return nil, err
	}
	if f.Nullable && typ != super.TypeNull {
		typ = r.sctx.LookupTypeUnion([]super.Type{typ, super.TypeNull})
	}
	return typ, nil
}

func (r *Reader) newUnionType(union arrow.UnionType) (super.Type, error) {
	var types []super.Type
	for _, f := range union.Fields() {
		typ, err := r.newTypeFromDataType(f.Type)
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
	superUnion := r.sctx.LookupTypeUnion(uniqueTypes)
	r.unionTagMappings[superUnion] = x
	return superUnion, nil
}

func (r *Reader) buildScodeWithNullable(typ super.Type, a arrow.Array, i int, nullable bool) error {
	if !nullable || typ == super.TypeNull {
		return r.buildScode(typ, a, i)
	}
	nullTag, nonNullTag, nonNullType := NullableUnionTagsAndType(typ.(*super.TypeUnion))
	if a.IsNull(i) {
		super.BuildUnion(&r.builder, nullTag, nil)
	} else {
		super.BeginUnion(&r.builder, nonNullTag)
		if err := r.buildScode(nonNullType, a, i); err != nil {
			return err
		}
		r.builder.EndContainer()
	}
	return nil
}

func NullableUnionTagsAndType(union *super.TypeUnion) (nullTag, nonNullTag int, nonNullType super.Type) {
	if len(union.Types) != 2 {
		panic(sup.FormatType(union))
	}
	if union.Types[0] == super.TypeNull {
		nullTag, nonNullTag = 0, 1
	} else {
		nullTag, nonNullTag = 1, 0
	}
	return nullTag, nonNullTag, union.Types[nonNullTag]
}

func (r *Reader) buildScode(typ super.Type, a arrow.Array, i int) error {
	b := &r.builder
	data := a.Data()
	dt := a.DataType()
	// XXX Calling array.New*Data once per value (rather than once
	// per arrow.Array) is slow.
	//
	// Order here follows that of the arrow.Type constants.
	switch dt.ID() {
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
		unit := dt.(*arrow.TimestampType).Unit
		b.Append(super.EncodeTime(nano.TimeToTs(array.NewTimestampData(data).Value(i).ToTime(unit))))
	case arrow.TIME32:
		unit := dt.(*arrow.Time32Type).Unit
		b.Append(super.EncodeTime(nano.TimeToTs(array.NewTime32Data(data).Value(i).ToTime(unit))))
	case arrow.TIME64:
		unit := dt.(*arrow.Time64Type).Unit
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
		return r.buildScodeList(typ, v, int(start), int(end))
	case arrow.STRUCT:
		v := array.NewStructData(data)
		arrowStructType := dt.(*arrow.StructType)
		superFields := typ.(*super.TypeRecord).Fields
		b.BeginContainer()
		for j := range v.NumField() {
			typ := superFields[j].Type
			nullable := arrowStructType.Field(j).Nullable
			if err := r.buildScodeWithNullable(typ, v.Field(j), i, nullable); err != nil {
				return err
			}
		}
		b.EndContainer()
	case arrow.SPARSE_UNION:
		return r.buildScodeUnion(typ, array.NewSparseUnionData(data), i)
	case arrow.DENSE_UNION:
		return r.buildScodeUnion(typ, array.NewDenseUnionData(data), i)
	case arrow.DICTIONARY:
		v := array.NewDictionaryData(data)
		return r.buildScode(typ, v.Dictionary(), v.GetValueIndex(i))
	case arrow.MAP:
		v := array.NewMapData(data)
		keys, items := v.Keys(), v.Items()
		mapType := typ.(*super.TypeMap)
		keyType, itemType := mapType.KeyType, mapType.ValType
		itemNullable := dt.(*arrow.MapType).ItemField().Nullable
		b.BeginContainer()
		for j, end := v.ValueOffsets(i); j < end; j++ {
			if err := r.buildScode(keyType, keys, int(j)); err != nil {
				return err
			}
			if err := r.buildScodeWithNullable(itemType, items, int(j), itemNullable); err != nil {
				return err
			}
		}
		b.TransformContainer(super.NormalizeMap)
		b.EndContainer()
	case arrow.FIXED_SIZE_LIST:
		v := array.NewFixedSizeListData(data)
		return r.buildScodeList(typ, v, 0, v.Len())
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
		return r.buildScodeList(typ, v, int(start), int(end))
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

func (r *Reader) buildScodeList(typ super.Type, a arrow.Array, start, end int) error {
	innerType := super.InnerType(typ)
	listValues := a.(array.ListLike).ListValues()
	nullable := a.DataType().(arrow.ListLikeType).ElemField().Nullable
	r.builder.BeginContainer()
	for i := start; i < end; i++ {
		if err := r.buildScodeWithNullable(innerType, listValues, i, nullable); err != nil {
			return err
		}
	}
	r.builder.EndContainer()
	return nil
}

func (r *Reader) buildScodeUnion(typ super.Type, u array.Union, i int) error {
	childID := u.ChildID(i)
	unionType := typ.(*super.TypeUnion)
	tag := r.unionTagMappings[unionType][childID]
	if u, ok := u.(*array.DenseUnion); ok {
		i = int(u.ValueOffset(i))
	}
	super.BeginUnion(&r.builder, tag)
	r.buildScode(unionType.Types[tag], u.Field(childID), i)
	r.builder.EndContainer()
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
