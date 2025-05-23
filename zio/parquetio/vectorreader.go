package parquetio

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio/arrowio"
	"golang.org/x/exp/constraints"
)

type VectorReader struct {
	ctx      context.Context
	sctx     *super.Context
	pushdown zbuf.Pushdown

	fr              *pqarrow.FileReader
	colIndexes      []int
	colIndexToField map[int]*pqarrow.SchemaField
	metadataFilter  expr.Evaluator
	nextRowGroup    *atomic.Int64
	rr              pqarrow.RecordReader
	vb              vectorBuilder
}

func NewVectorReader(ctx context.Context, sctx *super.Context, r io.Reader, pushdown zbuf.Pushdown) (*VectorReader, error) {
	ras, ok := r.(parquet.ReaderAtSeeker)
	if !ok {
		return nil, errors.New("reader cannot seek")
	}
	pr, err := file.NewParquetReader(ras)
	if err != nil {
		return nil, err
	}
	prmd := pr.MetaData()
	colIndexes := columnIndexes(prmd.Schema, pushdown.Projection().Paths())
	if len(colIndexes) == 0 {
		for i := range prmd.NumColumns() {
			colIndexes = append(colIndexes, i)
		}
	}
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
	var metadataFilter expr.Evaluator
	if pushdown != nil {
		metadataFilter, _, err = pushdown.MetaFilter()
		if err != nil {
			return nil, err
		}
	}
	return &VectorReader{
		ctx:             ctx,
		sctx:            sctx,
		pushdown:        pushdown,
		fr:              fr,
		colIndexes:      colIndexes,
		colIndexToField: schemaManifest.ColIndexToField,
		metadataFilter:  metadataFilter,
		nextRowGroup:    &atomic.Int64{},
		vb:              vectorBuilder{sctx, map[arrow.DataType]super.Type{}},
	}, nil
}

func (p *VectorReader) NewConcurrentPuller() (vector.Puller, error) {
	var metadataFilter expr.Evaluator
	if p.pushdown != nil {
		var err error
		metadataFilter, _, err = p.pushdown.MetaFilter()
		if err != nil {
			return nil, err
		}
	}
	return &VectorReader{
		ctx:             p.ctx,
		sctx:            p.sctx,
		fr:              p.fr,
		colIndexes:      p.colIndexes,
		colIndexToField: p.colIndexToField,
		metadataFilter:  metadataFilter,
		nextRowGroup:    p.nextRowGroup,
		vb:              vectorBuilder{p.sctx, map[arrow.DataType]super.Type{}},
	}, nil
}

func (p *VectorReader) Pull(done bool) (vector.Any, error) {
	if done {
		return nil, nil
	}
	for {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		if p.rr == nil {
			pr := p.fr.ParquetReader()
			rowGroup := int(p.nextRowGroup.Add(1) - 1)
			if rowGroup >= pr.NumRowGroups() {
				return nil, nil
			}
			if p.metadataFilter != nil {
				rgMetadata := pr.MetaData().RowGroup(rowGroup)
				val := buildMetadataValue(p.sctx, rgMetadata, p.colIndexes, p.colIndexToField)
				if !p.metadataFilter.Eval(nil, val).Ptr().AsBool() {
					continue
				}
			}
			rr, err := p.fr.GetRecordReader(p.ctx, p.colIndexes, []int{rowGroup})
			if err != nil {
				return nil, err
			}
			p.rr = rr
		}
		rec, err := p.rr.Read()
		if err != nil {
			if err == io.EOF {
				p.rr = nil
				continue
			}
			return nil, err
		}
		return p.vb.build(array.RecordToStructArray(rec))
	}
}

type vectorBuilder struct {
	sctx  *super.Context
	types map[arrow.DataType]super.Type
}

func (v *vectorBuilder) build(a arrow.Array) (vector.Any, error) {
	dt := a.DataType()
	length := uint32(a.Len())
	// For Boolean and numeric types, the runtime requires that null vector
	// slots contain the zero value.  This isn't always true for Arrow
	// vectors coming from pqarrow, so the code below must enforce it.
	nulls := makeNulls(a)
	// Order here follows that of the arrow.Type constants.
	switch dt.ID() {
	case arrow.NULL:
		bits := make([]uint64, (length+7)/8)
		for i := range bits {
			bits[i] = ^uint64(0)
		}
		nulls := bitvec.New(bits, length)
		return vector.NewConst(super.Null, length, nulls), nil
	case arrow.BOOL:
		vec := vector.NewBoolEmpty(length, nulls)
		arr := a.(*array.Boolean)
		for i := range length {
			if arr.Value(int(i)) && !nulls.IsSet(i) {
				vec.Set(i)
			}
		}
		return vec, nil
	case arrow.UINT8:
		values := convertSlice[uint64](a.(*array.Uint8).Uint8Values(), nulls)
		return vector.NewUint(super.TypeUint8, values, nulls), nil
	case arrow.INT8:
		values := convertSlice[int64](a.(*array.Int8).Int8Values(), nulls)
		return vector.NewInt(super.TypeInt8, values, nulls), nil
	case arrow.UINT16:
		values := convertSlice[uint64](a.(*array.Uint16).Uint16Values(), nulls)
		return vector.NewUint(super.TypeUint16, values, nulls), nil
	case arrow.INT16:
		values := convertSlice[int64](a.(*array.Int16).Int16Values(), nulls)
		return vector.NewInt(super.TypeInt16, values, nulls), nil
	case arrow.UINT32:
		values := convertSlice[uint64](a.(*array.Uint32).Uint32Values(), nulls)
		return vector.NewUint(super.TypeUint32, values, nulls), nil
	case arrow.INT32:
		values := convertSlice[int64](a.(*array.Int32).Int32Values(), nulls)
		return vector.NewInt(super.TypeInt32, values, nulls), nil
	case arrow.UINT64:
		values := a.(*array.Uint64).Uint64Values()
		zeroNulls(values, nulls)
		return vector.NewUint(super.TypeUint64, values, nulls), nil
	case arrow.INT64:
		values := a.(*array.Int64).Int64Values()
		zeroNulls(values, nulls)
		return vector.NewInt(super.TypeInt64, values, nulls), nil
	case arrow.FLOAT16:
		values := make([]float64, length)
		for i, v := range a.(*array.Float16).Values() {
			if !nulls.IsSet(uint32(i)) {
				values[i] = float64(v.Float32())
			}
		}
		return vector.NewFloat(super.TypeFloat16, values, nulls), nil
	case arrow.FLOAT32:
		values := convertSlice[float64](a.(*array.Float32).Float32Values(), nulls)
		return vector.NewFloat(super.TypeFloat32, values, nulls), nil
	case arrow.FLOAT64:
		values := a.(*array.Float64).Float64Values()
		zeroNulls(values, nulls)
		return vector.NewFloat(super.TypeFloat64, values, nulls), nil
	case arrow.STRING:
		arr := a.(*array.String)
		offsets := byteconv.ReinterpretSlice[uint32](arr.ValueOffsets())
		return vector.NewString(vector.NewBytesTable(offsets, arr.ValueBytes()), nulls), nil
	case arrow.BINARY:
		arr := a.(*array.Binary)
		offsets := byteconv.ReinterpretSlice[uint32](arr.ValueOffsets())
		return vector.NewBytes(vector.NewBytesTable(offsets, arr.ValueBytes()), nulls), nil
	case arrow.FIXED_SIZE_BINARY:
		value0 := a.(*array.FixedSizeBinary).Value(0)
		bytes := value0[:int(length)*len(value0)]
		offsets := make([]uint32, length+1)
		for i := range offsets {
			offsets[i] = uint32(i * len(value0))
		}
		return vector.NewBytes(vector.NewBytesTable(offsets, bytes), nulls), nil
	case arrow.DATE32:
		values := make([]int64, length)
		for i, v := range a.(*array.Date32).Date32Values() {
			if !nulls.IsSet(uint32(i)) {
				values[i] = int64(v) * int64(24*time.Hour)
			}
		}
		return vector.NewInt(super.TypeTime, values, nulls), nil
	case arrow.TIMESTAMP:
		multiplier := dt.(*arrow.TimestampType).TimeUnit().Multiplier()
		values := byteconv.ReinterpretSlice[int64](a.(*array.Timestamp).TimestampValues())
		if multiplier > 1 {
			for i := range values {
				values[i] *= int64(multiplier)
			}
		}
		zeroNulls(values, nulls)
		return vector.NewInt(super.TypeTime, values, nulls), nil
	case arrow.TIME32:
		multiplier := dt.(*arrow.Time32Type).TimeUnit().Multiplier()
		values := make([]int64, length)
		for i, v := range a.(*array.Time32).Time32Values() {
			if !nulls.IsSet(uint32(i)) {
				values[i] = int64(v) * int64(multiplier)
			}
		}
		return vector.NewInt(super.TypeTime, values, nulls), nil
	case arrow.TIME64:
		multiplier := dt.(*arrow.Time64Type).TimeUnit().Multiplier()
		values := byteconv.ReinterpretSlice[int64](a.(*array.Time64).Time64Values())
		if multiplier > 1 {
			for i := range values {
				values[i] *= int64(multiplier)
			}
		}
		zeroNulls(values, nulls)
		return vector.NewInt(super.TypeTime, values, nulls), nil
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
			if !nulls.IsSet(uint32(i)) {
				values[i] = v.ToFloat64(scale)
			}
		}
		return vector.NewFloat(typ, values, nulls), nil
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
			if !nulls.IsSet(uint32(i)) {
				values[i] = v.ToFloat64(scale)
			}
		}
		return vector.NewFloat(typ, values, nulls), nil
	case arrow.LIST:
		arr := a.(*array.List)
		values, err := v.build(arr.ListValues())
		if err != nil {
			return nil, err
		}
		offsets := byteconv.ReinterpretSlice[uint32](arr.Offsets())
		typ, ok := v.types[dt]
		if !ok {
			typ = v.sctx.LookupTypeArray(values.Type())
			v.types[dt] = typ
		}
		return vector.NewArray(typ.(*super.TypeArray), offsets, values, nulls), nil
	case arrow.STRUCT:
		arr := a.(*array.Struct)
		fieldVecs := make([]vector.Any, arr.NumField())
		for i := range arr.NumField() {
			vec, err := v.build(arr.Field(i))
			if err != nil {
				return nil, err
			}
			fieldVecs[i] = vec
		}
		typ, ok := v.types[dt]
		if !ok {
			fields := make([]super.Field, arr.NumField())
			for i, vec := range fieldVecs {
				fields[i] = super.NewField(dt.(*arrow.StructType).Field(i).Name, vec.Type())
			}
			arrowio.UniquifyFieldNames(fields)
			var err error
			typ, err = v.sctx.LookupTypeRecord(fields)
			if err != nil {
				return nil, err
			}
			v.types[dt] = typ
		}
		return vector.NewRecord(typ.(*super.TypeRecord), fieldVecs, length, nulls), nil
	// case arrow.MAP: TODO
	case arrow.FIXED_SIZE_LIST:
		arr := a.(*array.FixedSizeList)
		values, err := v.build(arr.ListValues())
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
			typ = v.sctx.LookupTypeArray(values.Type())
			v.types[dt] = typ
		}
		return vector.NewArray(typ.(*super.TypeArray), offsets, values, nulls), nil
	case arrow.LARGE_STRING:
		arr := a.(*array.LargeString)
		offsets := convertSlice[uint32](arr.ValueOffsets(), bitvec.Zero)
		for i, o := range arr.ValueOffsets() {
			if int64(offsets[i]) != o {
				return nil, fmt.Errorf("string offset exceeds uint32 range")
			}
		}
		return vector.NewString(vector.NewBytesTable(offsets, arr.ValueBytes()), nulls), nil

	}
	return nil, fmt.Errorf("unimplemented Parquet type %q", dt.Name())
}

func makeNulls(a arrow.Array) bitvec.Bits {
	bytes := a.NullBitmapBytes()
	if len(bytes) == 0 {
		return bitvec.Zero
	}
	n := a.Len()
	bits := make([]uint64, (n+63)/64)
	bitsAsBytes := byteconv.ReinterpretSlice[byte](bits)
	copy(bitsAsBytes, bytes)
	for i := range bits {
		// Flip bits
		bits[i] ^= ^uint64(0)
	}
	return bitvec.New(bits, uint32(n))
}

func convertSlice[Out, In constraints.Float | constraints.Integer](in []In, nulls bitvec.Bits) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		if !nulls.IsSet(uint32(i)) {
			out[i] = Out(v)
		}
	}
	return out
}

func zeroNulls[T any](s []T, nulls bitvec.Bits) {
	var zero T
	for i := range s {
		if nulls.IsSet(uint32(i)) {
			s[i] = zero
		}
	}
}
