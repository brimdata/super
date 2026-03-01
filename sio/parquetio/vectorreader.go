package parquetio

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync/atomic"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/arrowio"
	"github.com/brimdata/super/vector"
	"golang.org/x/exp/constraints"
)

type VectorReader struct {
	ctx  context.Context
	sctx *super.Context

	fr                 *pqarrow.FileReader
	colIndexes         []int
	colIndexToField    map[int]*pqarrow.SchemaField
	metadataColIndexes []int
	metadataFilters    []expr.Evaluator

	nextRowGroup *atomic.Int64
	rrs          []pqarrow.RecordReader
	vbs          []vectorBuilder
}

func NewVectorReader(ctx context.Context, sctx *super.Context, r io.Reader, p sbuf.Pushdown, concurrentReaders int) (*VectorReader, error) {
	if concurrentReaders < 1 {
		panic(concurrentReaders)
	}
	ras, ok := r.(parquet.ReaderAtSeeker)
	if !ok {
		return nil, errNotSeekable
	}
	pr, err := file.NewParquetReader(ras)
	if err != nil {
		return nil, err
	}
	prmd := pr.MetaData()
	pqprops := pqarrow.ArrowReadProperties{
		Parallel:  true,
		BatchSize: 16184,
	}
	schemaManifest, err := pqarrow.NewSchemaManifest(prmd.Schema, prmd.KeyValueMetadata(), &pqprops)
	if err != nil {
		return nil, err
	}
	fr, err := pqarrow.NewFileReader(pr, pqprops, memory.NewGoAllocator())
	if err != nil {
		return nil, err
	}
	var metadataColIndexes []int
	var metadataFilters []expr.Evaluator
	if p != nil {
		filter, projection, err := p.MetaFilter()
		if err != nil {
			return nil, err
		}
		if filter != nil {
			paths := projection.Paths()
			for i, p := range paths {
				// Trim trailing "max" or "min".
				paths[i] = p[:len(p)-1]
			}
			colIndexes := columnIndexes(pr.MetaData().Schema, paths)
			// Remove duplicates created above by trimming "max" and "min".
			metadataColIndexes = slices.Compact(colIndexes)
			for range concurrentReaders {
				filter, _, err := p.MetaFilter()
				if err != nil {
					return nil, err
				}
				metadataFilters = append(metadataFilters, filter)
			}
		}
	}
	var vbs []vectorBuilder
	for range concurrentReaders {
		vbs = append(vbs, vectorBuilder{sctx, map[arrow.DataType]super.Type{}})
	}
	return &VectorReader{
		ctx:                ctx,
		sctx:               sctx,
		fr:                 fr,
		colIndexes:         columnIndexes(prmd.Schema, p.Projection().Paths()),
		colIndexToField:    schemaManifest.ColIndexToField,
		metadataColIndexes: metadataColIndexes,
		metadataFilters:    metadataFilters,
		nextRowGroup:       &atomic.Int64{},
		rrs:                make([]pqarrow.RecordReader, concurrentReaders),
		vbs:                vbs,
	}, nil
}

func (p *VectorReader) Pull(done bool) (vector.Any, error) {
	return p.ConcurrentPull(done, 0)
}

func (p *VectorReader) ConcurrentPull(done bool, id int) (vector.Any, error) {
	if done {
		return nil, nil
	}
	for {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		if p.rrs[id] == nil {
			pr := p.fr.ParquetReader()
			rowGroup := int(p.nextRowGroup.Add(1) - 1)
			if rowGroup >= pr.NumRowGroups() {
				return nil, nil
			}
			if len(p.metadataFilters) > 0 {
				rgMetadata := pr.MetaData().RowGroup(rowGroup)
				val := buildMetadataValue(p.sctx, rgMetadata, p.metadataColIndexes, p.colIndexToField)
				if !p.metadataFilters[id].Eval(val).Ptr().AsBool() {
					continue
				}
			}
			rr, err := p.fr.GetRecordReader(p.ctx, p.colIndexes, []int{rowGroup})
			if err != nil {
				return nil, err
			}
			p.rrs[id] = rr
		}
		batch, err := p.rrs[id].Read()
		if err != nil {
			if err == io.EOF {
				p.rrs[id] = nil
				continue
			}
			return nil, err
		}
		return p.vbs[id].build(array.RecordToStructArray(batch), false)
	}
}

type vectorBuilder struct {
	sctx  *super.Context
	types map[arrow.DataType]super.Type
}

func (v *vectorBuilder) build(a arrow.Array, nullable bool) (vector.Any, error) {
	dt := a.DataType()
	length := uint32(a.Len())
	var out vector.Any
	// Order here follows that of the arrow.Type constants.
	switch dt.ID() {
	case arrow.NULL:
		return vector.NewConst(super.Null, length), nil
	case arrow.BOOL:
		vec := vector.NewFalse(length)
		arr := a.(*array.Boolean)
		for i := range length {
			if arr.Value(int(i)) {
				vec.Set(i)
			}
		}
		out = vec
	case arrow.UINT8:
		values := convertSlice[uint64](a.(*array.Uint8).Uint8Values())
		out = vector.NewUint(super.TypeUint8, values)
	case arrow.INT8:
		values := convertSlice[int64](a.(*array.Int8).Int8Values())
		out = vector.NewInt(super.TypeInt8, values)
	case arrow.UINT16:
		values := convertSlice[uint64](a.(*array.Uint16).Uint16Values())
		out = vector.NewUint(super.TypeUint16, values)
	case arrow.INT16:
		values := convertSlice[int64](a.(*array.Int16).Int16Values())
		out = vector.NewInt(super.TypeInt16, values)
	case arrow.UINT32:
		values := convertSlice[uint64](a.(*array.Uint32).Uint32Values())
		out = vector.NewUint(super.TypeUint32, values)
	case arrow.INT32:
		values := convertSlice[int64](a.(*array.Int32).Int32Values())
		out = vector.NewInt(super.TypeInt32, values)
	case arrow.UINT64:
		values := a.(*array.Uint64).Uint64Values()
		out = vector.NewUint(super.TypeUint64, values)
	case arrow.INT64:
		values := a.(*array.Int64).Int64Values()
		out = vector.NewInt(super.TypeInt64, values)
	case arrow.FLOAT16:
		values := make([]float64, length)
		for i, v := range a.(*array.Float16).Values() {
			values[i] = float64(v.Float32())
		}
		out = vector.NewFloat(super.TypeFloat16, values)
	case arrow.FLOAT32:
		values := convertSlice[float64](a.(*array.Float32).Float32Values())
		out = vector.NewFloat(super.TypeFloat32, values)
	case arrow.FLOAT64:
		values := a.(*array.Float64).Float64Values()
		out = vector.NewFloat(super.TypeFloat64, values)
	case arrow.STRING:
		arr := a.(*array.String)
		offsets := byteconv.ReinterpretSlice[uint32](arr.ValueOffsets())
		out = vector.NewString(vector.NewBytesTable(offsets, arr.ValueBytes()))
	case arrow.BINARY:
		arr := a.(*array.Binary)
		offsets := byteconv.ReinterpretSlice[uint32](arr.ValueOffsets())
		out = vector.NewBytes(vector.NewBytesTable(offsets, arr.ValueBytes()))
	case arrow.FIXED_SIZE_BINARY:
		value0 := a.(*array.FixedSizeBinary).Value(0)
		bytes := value0[:int(length)*len(value0)]
		offsets := make([]uint32, length+1)
		for i := range offsets {
			offsets[i] = uint32(i * len(value0))
		}
		out = vector.NewBytes(vector.NewBytesTable(offsets, bytes))
	case arrow.DATE32:
		values := make([]int64, length)
		for i, v := range a.(*array.Date32).Date32Values() {
			values[i] = int64(v) * int64(24*time.Hour)
		}
		out = vector.NewInt(super.TypeTime, values)
	case arrow.TIMESTAMP:
		multiplier := dt.(*arrow.TimestampType).TimeUnit().Multiplier()
		values := byteconv.ReinterpretSlice[int64](a.(*array.Timestamp).TimestampValues())
		if multiplier > 1 {
			for i := range values {
				values[i] *= int64(multiplier)
			}
		}
		out = vector.NewInt(super.TypeTime, values)
	case arrow.TIME32:
		multiplier := dt.(*arrow.Time32Type).TimeUnit().Multiplier()
		values := make([]int64, length)
		for i, v := range a.(*array.Time32).Time32Values() {
			values[i] = int64(v) * int64(multiplier)
		}
		out = vector.NewInt(super.TypeTime, values)
	case arrow.TIME64:
		multiplier := dt.(*arrow.Time64Type).TimeUnit().Multiplier()
		values := byteconv.ReinterpretSlice[int64](a.(*array.Time64).Time64Values())
		if multiplier > 1 {
			for i := range values {
				values[i] *= int64(multiplier)
			}
		}
		out = vector.NewInt(super.TypeTime, values)
	case arrow.DECIMAL128:
		typ, ok := v.types[dt]
		if !ok {
			d := dt.(arrow.DecimalType)
			name := fmt.Sprintf("deciaml_%d_%d", d.GetScale(), d.GetPrecision())
			var err error
			typ, err = v.sctx.LookupTypeNamed(name, super.TypeFloat64)
			if err != nil {
				return nil, err
			}
			v.types[dt] = typ
		}
		scale := dt.(arrow.DecimalType).GetScale()
		values := make([]float64, length)
		for i, v := range a.(*array.Decimal128).Values() {
			values[i] = v.ToFloat64(scale)
		}
		out = vector.NewFloat(typ, values)
	case arrow.DECIMAL256:
		typ, ok := v.types[dt]
		if !ok {
			d := dt.(arrow.DecimalType)
			name := fmt.Sprintf("deciaml_%d_%d", d.GetScale(), d.GetPrecision())
			var err error
			typ, err = v.sctx.LookupTypeNamed(name, super.TypeFloat64)
			if err != nil {
				return nil, err
			}
			v.types[dt] = typ
		}
		scale := dt.(arrow.DecimalType).GetScale()
		values := make([]float64, length)
		for i, v := range a.(*array.Decimal256).Values() {
			values[i] = v.ToFloat64(scale)
		}
		out = vector.NewFloat(typ, values)
	case arrow.LIST:
		arr := a.(*array.List)
		nullable := dt.(*arrow.ListType).ElemField().Nullable
		values, err := v.build(arr.ListValues(), nullable)
		if err != nil {
			return nil, err
		}
		offsets := byteconv.ReinterpretSlice[uint32](arr.Offsets())
		typ, ok := v.types[dt]
		if !ok {
			inner := values.Type()
			if nullable {
				inner = v.sctx.LookupTypeUnion([]super.Type{inner, super.TypeNull})
			}
			typ = v.sctx.LookupTypeArray(inner)
			v.types[dt] = typ
		}
		out = vector.NewArray(typ.(*super.TypeArray), offsets, values)
	case arrow.STRUCT:
		arr := a.(*array.Struct)
		arrowStructType := dt.(*arrow.StructType)
		fieldVecs := make([]vector.Any, arr.NumField())
		for i := range arr.NumField() {
			vec, err := v.build(arr.Field(i), arrowStructType.Field(i).Nullable)
			if err != nil {
				return nil, err
			}
			fieldVecs[i] = vec
		}
		typ, ok := v.types[dt]
		if !ok {
			fields := make([]super.Field, arr.NumField())
			for i, vec := range fieldVecs {
				arrowField := arrowStructType.Field(i)
				typ := vec.Type()
				if arrowField.Nullable {
					typ = v.sctx.LookupTypeUnion([]super.Type{typ, super.TypeNull})
				}
				fields[i] = super.NewField(arrowField.Name, typ)
			}
			arrowio.UniquifyFieldNames(fields)
			var err error
			typ, err = v.sctx.LookupTypeRecord(fields)
			if err != nil {
				return nil, err
			}
			v.types[dt] = typ
		}
		out = vector.NewRecord(typ.(*super.TypeRecord), fieldVecs, length)
	// case arrow.MAP: TODO
	case arrow.FIXED_SIZE_LIST:
		arr := a.(*array.FixedSizeList)
		nullable := dt.(*arrow.FixedSizeListType).ElemField().Nullable
		values, err := v.build(arr.ListValues(), nullable)
		if err != nil {
			return nil, err
		}
		listLen := dt.(*arrow.FixedSizeListType).Len()
		offsets := make([]uint32, length+1)
		for i := range offsets {
			offsets[i] = uint32(i) * uint32(listLen)
		}
		typ, ok := v.types[dt]
		if !ok {
			inner := values.Type()
			if nullable {
				inner = v.sctx.LookupTypeUnion([]super.Type{inner, super.TypeNull})
			}
			typ = v.sctx.LookupTypeArray(inner)
			v.types[dt] = typ
		}
		out = vector.NewArray(typ.(*super.TypeArray), offsets, values)
	case arrow.LARGE_STRING:
		arr := a.(*array.LargeString)
		offsets := convertSlice[uint32](arr.ValueOffsets())
		for i, o := range arr.ValueOffsets() {
			if int64(offsets[i]) != o {
				return nil, fmt.Errorf("string offset exceeds uint32 range")
			}
		}
		out = vector.NewString(vector.NewBytesTable(offsets, arr.ValueBytes()))
	default:
		return nil, fmt.Errorf("unimplemented Parquet type %q", dt.Name())
	}
	if nullable {
		return v.buildNullableUnion(out, a), nil
	}
	return out, nil
}

func (v *vectorBuilder) buildNullableUnion(vec vector.Any, a arrow.Array) vector.Any {
	unionType := v.sctx.LookupTypeUnion([]super.Type{vec.Type(), super.TypeNull})
	nullTag, vecTag, _ := arrowio.NullableUnionTagsAndType(unionType)
	tags := make([]uint32, vec.Len())
	var vecIndex []uint32
	for i := range vec.Len() {
		if a.IsNull(int(i)) {
			tags[i] = uint32(nullTag)
		} else {
			tags[i] = uint32(vecTag)
			vecIndex = append(vecIndex, i)
		}
	}
	var vecs [2]vector.Any
	vecs[nullTag] = vector.NewConst(super.Null, vec.Len()-uint32(len(vecIndex)))
	vecs[vecTag] = vector.Pick(vec, vecIndex)
	return vector.NewUnion(unionType, tags, vecs[:])
}

func convertSlice[Out, In constraints.Float | constraints.Integer](in []In) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = Out(v)
	}
	return out
}
