package parquetio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zio/arrowio"
)

type VectorReader struct {
	ctx  context.Context
	zctx *super.Context

	fr           *pqarrow.FileReader
	colIndexes   []int
	nextRowGroup *atomic.Int64
	rr           pqarrow.RecordReader
	vb           vectorBuilder
}

func NewVectorReader(ctx context.Context, zctx *super.Context, r io.Reader, fields []field.Path) (*VectorReader, error) {
	ras, ok := r.(parquet.ReaderAtSeeker)
	if !ok {
		return nil, errors.New("reader cannot seek")
	}
	pr, err := file.NewParquetReader(ras)
	if err != nil {
		return nil, err
	}
	pqprops := pqarrow.ArrowReadProperties{
		Parallel:  true,
		BatchSize: 16184,
	}
	fr, err := pqarrow.NewFileReader(pr, pqprops, memory.NewGoAllocator())
	if err != nil {

	}
	return &VectorReader{
		ctx:          ctx,
		zctx:         zctx,
		fr:           fr,
		colIndexes:   columnIndexes(fr.Manifest, fields),
		nextRowGroup: &atomic.Int64{},
		vb:           vectorBuilder{zctx, map[arrow.DataType]super.Type{}},
	}, nil
}

func (p *VectorReader) NewConcurrentPuller() vector.Puller {
	return &VectorReader{
		ctx:          p.ctx,
		fr:           p.fr,
		colIndexes:   p.colIndexes,
		nextRowGroup: p.nextRowGroup,
		vb:           vectorBuilder{p.zctx, map[arrow.DataType]super.Type{}},
	}
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
			rowGroup := int(p.nextRowGroup.Add(1) - 1)
			if rowGroup >= p.fr.ParquetReader().NumRowGroups() {
				return nil, nil
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
	zctx  *super.Context
	types map[arrow.DataType]super.Type
}

func (v *vectorBuilder) build(a arrow.Array) (vector.Any, error) {
	dt := a.DataType()
	length := uint32(a.Len())
	// Order here follows that of the arrow.Type constants.
	switch dt.ID() {
	case arrow.NULL:
		bits := make([]uint64, (length+7)/8)
		for i := range bits {
			bits[i] = ^uint64(0)
		}
		nulls := vector.NewBool(bits, length, nil)
		return vector.NewConst(super.Null, length, nulls), nil
	case arrow.BOOL:
		vec := vector.NewBoolEmpty(length, makeNulls(a))
		arr := a.(*array.Boolean)
		for i := range length {
			if arr.Value(int(i)) {
				vec.Set(i)
			}
		}
		return vec, nil
	case arrow.UINT8:
		values := convertSlice[uint64](a.(*array.Uint8).Uint8Values())
		return vector.NewUint(super.TypeUint8, values, makeNulls(a)), nil
	case arrow.INT8:
		values := convertSlice[int64](a.(*array.Int8).Int8Values())
		return vector.NewInt(super.TypeInt8, values, makeNulls(a)), nil
	case arrow.UINT16:
		values := convertSlice[uint64](a.(*array.Uint16).Uint16Values())
		return vector.NewUint(super.TypeUint16, values, makeNulls(a)), nil
	case arrow.INT16:
		values := convertSlice[int64](a.(*array.Int16).Int16Values())
		return vector.NewInt(super.TypeInt16, values, makeNulls(a)), nil
	case arrow.UINT32:
		values := convertSlice[uint64](a.(*array.Uint32).Uint32Values())
		return vector.NewUint(super.TypeUint32, values, makeNulls(a)), nil
	case arrow.INT32:
		values := convertSlice[int64](a.(*array.Int32).Int32Values())
		return vector.NewInt(super.TypeInt32, values, makeNulls(a)), nil
	case arrow.UINT64:
		values := a.(*array.Uint64).Uint64Values()
		return vector.NewUint(super.TypeUint64, values, makeNulls(a)), nil
	case arrow.INT64:
		values := a.(*array.Int64).Int64Values()
		return vector.NewInt(super.TypeInt64, values, makeNulls(a)), nil
	case arrow.FLOAT16:
		values := make([]float64, length)
		for i, v := range a.(*array.Float16).Values() {
			values[i] = float64(v.Float32())
		}
		return vector.NewFloat(super.TypeFloat16, values, makeNulls(a)), nil
	case arrow.FLOAT32:
		values := convertSlice[float64](a.(*array.Float32).Float32Values())
		return vector.NewFloat(super.TypeFloat32, values, makeNulls(a)), nil
	case arrow.FLOAT64:
		values := a.(*array.Float64).Float64Values()
		return vector.NewFloat(super.TypeFloat64, values, makeNulls(a)), nil
	case arrow.STRING:
		arr := a.(*array.String)
		offsets := reinterpretSlice[uint32](arr.ValueOffsets())
		return vector.NewString(offsets, arr.ValueBytes(), makeNulls(a)), nil
	case arrow.BINARY:
		arr := a.(*array.Binary)
		offsets := reinterpretSlice[uint32](arr.ValueOffsets())
		return vector.NewBytes(offsets, arr.ValueBytes(), makeNulls(a)), nil
	case arrow.FIXED_SIZE_BINARY:
		value0 := a.(*array.FixedSizeBinary).Value(0)
		bytes := value0[:int(length)*len(value0)]
		offsets := make([]uint32, length+1)
		for i := range offsets {
			offsets[i] = uint32(i * len(value0))
		}
		return vector.NewBytes(offsets, bytes, makeNulls(a)), nil
	case arrow.DATE32:
		values := make([]int64, length)
		for i, v := range a.(*array.Date32).Date32Values() {
			values[i] = int64(v) * int64(24*time.Hour)
		}
		return vector.NewInt(super.TypeTime, values, makeNulls(a)), nil
	case arrow.TIMESTAMP:
		multiplier := dt.(*arrow.TimestampType).TimeUnit().Multiplier()
		values := reinterpretSlice[int64](a.(*array.Timestamp).TimestampValues())
		if multiplier > 1 {
			for i := range values {
				values[i] *= int64(multiplier)
			}
		}
		return vector.NewInt(super.TypeTime, values, makeNulls(a)), nil
	case arrow.TIME32:
		multiplier := dt.(*arrow.Time32Type).TimeUnit().Multiplier()
		values := make([]int64, length)
		for i, v := range a.(*array.Time32).Time32Values() {
			values[i] = int64(v) * int64(multiplier)
		}
		return vector.NewInt(super.TypeTime, values, makeNulls(a)), nil
	case arrow.TIME64:
		multiplier := dt.(*arrow.Time64Type).TimeUnit().Multiplier()
		values := reinterpretSlice[int64](a.(*array.Time64).Time64Values())
		if multiplier > 1 {
			for i := range values {
				values[i] *= int64(multiplier)
			}
		}
		return vector.NewInt(super.TypeTime, values, makeNulls(a)), nil
	case arrow.DECIMAL128:
		typ, ok := v.types[dt]
		if !ok {
			d := dt.(arrow.DecimalType)
			name := fmt.Sprintf("deciaml_%d_%d", d.GetScale(), d.GetPrecision())
			var err error
			typ, err = v.zctx.LookupTypeNamed(name, super.TypeFloat64)
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
		return vector.NewFloat(typ, values, makeNulls(a)), nil
	case arrow.DECIMAL256:
		typ, ok := v.types[dt]
		if !ok {
			d := dt.(arrow.DecimalType)
			name := fmt.Sprintf("deciaml_%d_%d", d.GetScale(), d.GetPrecision())
			var err error
			typ, err = v.zctx.LookupTypeNamed(name, super.TypeFloat64)
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
		return vector.NewFloat(typ, values, makeNulls(a)), nil
	case arrow.LIST:
		arr := a.(*array.List)
		values, err := v.build(arr.ListValues())
		if err != nil {
			return nil, err
		}
		offsets := reinterpretSlice[uint32](arr.Offsets())
		typ, ok := v.types[dt]
		if !ok {
			typ = v.zctx.LookupTypeArray(values.Type())
			v.types[dt] = typ
		}
		return vector.NewArray(typ.(*super.TypeArray), offsets, values, makeNulls(a)), nil
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
			typ, err = v.zctx.LookupTypeRecord(fields)
			if err != nil {
				return nil, err
			}
			v.types[dt] = typ
		}
		return vector.NewRecord(typ.(*super.TypeRecord), fieldVecs, length, makeNulls(a)), nil
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
			typ = v.zctx.LookupTypeArray(values.Type())
			v.types[dt] = typ
		}
		return vector.NewArray(typ.(*super.TypeArray), offsets, values, makeNulls(a)), nil
	case arrow.LARGE_STRING:
		arr := a.(*array.LargeString)
		offsets := convertSlice[uint32](arr.ValueOffsets())
		for i, o := range arr.ValueOffsets() {
			if int64(offsets[i]) != o {
				return nil, fmt.Errorf("string offset exceeds uint32 range")
			}
		}
		return vector.NewString(offsets, arr.ValueBytes(), makeNulls(a)), nil

	}
	return nil, fmt.Errorf("unimplemented Parquet type %q", dt.Name())
}

func makeNulls(a arrow.Array) *vector.Bool {
	bytes := a.NullBitmapBytes()
	if len(bytes) == 0 {
		return nil
	}
	bits := make([]uint64, (len(bytes)+7)/8)
	bitsAsBytes := reinterpretSlice[byte](bits)
	copy(bitsAsBytes, bytes)
	for i := range bits {
		// Flip bits
		bits[i] ^= ^uint64(0)
	}
	return vector.NewBool(bits, uint32(a.Len()), nil)
}

func convertSlice[Out, In uint8 | uint16 | uint32 | uint64 | int8 | int16 | int32 | int64 | float32 | float64](in []In) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = Out(v)
	}
	return out
}

func reinterpretSlice[Out, In any](in []In) []Out {
	outData := (*Out)(unsafe.Pointer(unsafe.SliceData(in)))
	outLen := len(in) * int(unsafe.Sizeof(in[0])) / int(unsafe.Sizeof(*outData))
	outCap := cap(in) * int(unsafe.Sizeof(in[0])) / int(unsafe.Sizeof(*outData))
	return unsafe.Slice(outData, outCap)[:outLen]
}
