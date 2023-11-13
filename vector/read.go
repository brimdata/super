package vector

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler/optimizer/demand"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/vng"
	vngvector "github.com/brimdata/zed/vng/vector"
	"github.com/brimdata/zed/zcode"
)

func Read(object *vng.Object, demandOut demand.Demand) (*Vector, error) {
	var buf []byte
	tags, err := ReadInt64s(object.ReaderAt, &buf, object.Root)
	if err != nil {
		return nil, err
	}
	types := make([]zed.Type, len(object.Maps))
	values := make([]vector, len(object.Maps))
	for i, metadata := range object.Maps {
		typeCopy := metadata.Type(object.Zctx)
		typ := typeAfterDemand(object.Zctx, metadata, demandOut, typeCopy)
		types[i] = typ
		value, err := read(object.Zctx, object.ReaderAt, &buf, metadata, demandOut)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	vector := &Vector{
		Context: object.Zctx,
		Types:   types,
		values:  values,
		tags:    tags,
	}
	return vector, nil
}

func read(zctx *zed.Context, readerAt io.ReaderAt, buf *[]byte, meta vngvector.Metadata, demandOut demand.Demand) (vector, error) {
	if demand.IsNone(demandOut) {
		return &constants{}, nil
	}

	switch meta := meta.(type) {

	case *vngvector.Array:
		lengths, err := ReadInt64s(readerAt, buf, meta.Lengths)
		if err != nil {
			return nil, err
		}
		elems, err := read(zctx, readerAt, buf, meta.Values, demand.All())
		if err != nil {
			return nil, err
		}
		vector := &arrays{
			lengths: lengths,
			elems:   elems,
		}
		return vector, nil

	case *vngvector.Const:
		vector := &constants{
			bytes: meta.Value.Bytes(),
		}
		return vector, nil

	case *vngvector.Map:
		keys, err := read(zctx, readerAt, buf, meta.Keys, demand.All())
		if err != nil {
			return nil, err
		}
		lengths, err := ReadInt64s(readerAt, buf, meta.Lengths)
		if err != nil {
			return nil, err
		}
		values, err := read(zctx, readerAt, buf, meta.Values, demand.All())
		if err != nil {
			return nil, err
		}
		vector := &maps{
			lengths: lengths,
			keys:    keys,
			values:  values,
		}
		return vector, nil

	case *vngvector.Nulls:
		runs, err := ReadInt64s(readerAt, buf, meta.Runs)
		if err != nil {
			return nil, err
		}
		values, err := read(zctx, readerAt, buf, meta.Values, demandOut)
		if err != nil {
			return nil, err
		}
		if len(runs) == 0 {
			return values, nil
		}
		vector := &nulls{
			runs:   runs,
			values: values,
		}
		return vector, nil

	case *vngvector.Primitive:
		if len(meta.Dict) != 0 {
			tags, err := readSegmap(readerAt, meta.Segmap)
			if err != nil {
				return nil, err
			}
			return &dict{
				dict: meta.Dict,
				tags: tags,
			}, nil
		} else {
			return readPrimitive(zctx, readerAt, buf, meta.Segmap, meta.Type(zctx))
		}

	case *vngvector.Record:
		var fields []vector
		for _, fieldMeta := range meta.Fields {
			demandValueOut := demand.GetKey(demandOut, fieldMeta.Name)
			if !demand.IsNone(demandValueOut) {
				field, err := read(zctx, readerAt, buf, fieldMeta.Values, demandValueOut)
				if err != nil {
					return nil, err
				}
				fields = append(fields, field)
			}
		}
		vector := &records{
			fields: fields,
		}
		return vector, nil

	case *vngvector.Set:
		lengths, err := ReadInt64s(readerAt, buf, meta.Lengths)
		if err != nil {
			return nil, err
		}
		elems, err := read(zctx, readerAt, buf, meta.Values, demand.All())
		if err != nil {
			return nil, err
		}
		vector := &sets{
			lengths: lengths,
			elems:   elems,
		}
		return vector, nil

	case *vngvector.Union:
		payloads := make([]vector, len(meta.Values))
		for i, valueMeta := range meta.Values {
			payload, err := read(zctx, readerAt, buf, valueMeta, demandOut)
			if err != nil {
				return nil, err
			}
			payloads[i] = payload
		}
		tags, err := ReadInt64s(readerAt, buf, meta.Tags)
		if err != nil {
			return nil, err
		}
		vector := &unions{
			payloads: payloads,
			tags:     tags,
		}
		return vector, nil

	default:
		return nil, fmt.Errorf("unknown VNG meta type: %T", meta)
	}
}

func ReadInt64s(readerAt io.ReaderAt, buf *[]byte, segmap []vngvector.Segment) ([]int64, error) {
	vector, err := readPrimitive(nil, readerAt, buf, segmap, zed.TypeInt64)
	if err != nil {
		return nil, err
	}
	return vector.(*int64s).values, nil
}

var errBadTag = errors.New("bad tag")

func readPrimitive(zctx *zed.Context, readerAt io.ReaderAt, buf *[]byte, segmap []vngvector.Segment, typ zed.Type) (vector, error) {
	var count int
	for _, segment := range segmap {
		count += int(segment.Count)
	}

	switch typ {
	case zed.TypeBool:
		values := make([]bool, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				value := zed.DecodeBool(bs)
				values = append(values, value)
			}
		}
		vector := &bools{
			values: values,
		}
		return vector, nil

	case zed.TypeBytes:
		data, err := readSegmap(readerAt, segmap)
		if err != nil {
			return nil, err
		}
		data, offsets, err := stripContainers(data, count)
		if err != nil {
			return nil, err
		}
		vector := &byteses{
			data:    data,
			offsets: offsets,
		}
		return vector, nil

	case zed.TypeDuration:
		values := make([]nano.Duration, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, zed.DecodeDuration(bs))
			}
		}
		vector := &durations{
			values: values,
		}
		return vector, nil

	case zed.TypeFloat16:
		values := make([]float32, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, zed.DecodeFloat16(bs))
			}
		}
		vector := &float16s{
			values: values,
		}
		return vector, nil

	case zed.TypeFloat32:
		values := make([]float32, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, zed.DecodeFloat32(bs))
			}
		}
		vector := &float32s{
			values: values,
		}
		return vector, nil

	case zed.TypeFloat64:
		values := make([]float64, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, zed.DecodeFloat64(bs))
			}
		}
		vector := &float64s{
			values: values,
		}
		return vector, nil

	case zed.TypeInt8:
		values := make([]int8, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				value := int8(zed.DecodeInt(bs))
				values = append(values, value)
			}
		}
		vector := &int8s{
			values: values,
		}
		return vector, nil

	case zed.TypeInt16:
		values := make([]int16, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				value := int16(zed.DecodeInt(bs))
				values = append(values, value)
			}
		}
		vector := &int16s{
			values: values,
		}
		return vector, nil

	case zed.TypeInt32:
		values := make([]int32, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				value := int32(zed.DecodeInt(bs))
				values = append(values, value)
			}
		}
		vector := &int32s{
			values: values,
		}
		return vector, nil

	case zed.TypeInt64:
		values := make([]int64, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				value := int64(zed.DecodeInt(bs))
				values = append(values, value)
			}
		}
		vector := &int64s{
			values: values,
		}
		return vector, nil

	case zed.TypeIP:
		values := make([]netip.Addr, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, zed.DecodeIP(bs))
			}
		}
		vector := &ips{
			values: values,
		}
		return vector, nil

	case zed.TypeNet:
		values := make([]netip.Prefix, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, zed.DecodeNet(bs))
			}
		}
		vector := &nets{
			values: values,
		}
		return vector, nil

	case zed.TypeString:
		data, err := readSegmap(readerAt, segmap)
		if err != nil {
			return nil, err
		}
		data, offsets, err := stripContainers(data, count)
		if err != nil {
			return nil, err
		}
		vector := &strings{
			data:    data,
			offsets: offsets,
		}
		return vector, nil

	case zed.TypeTime:
		values := make([]nano.Ts, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, zed.DecodeTime(bs))
			}
		}
		vector := &times{
			values: values,
		}
		return vector, nil

	case zed.TypeUint8:
		values := make([]uint8, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, uint8(zed.DecodeUint(bs)))
			}
		}
		vector := &uint8s{
			values: values,
		}
		return vector, nil

	case zed.TypeUint16:
		values := make([]uint16, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, uint16(zed.DecodeUint(bs)))
			}
		}
		vector := &uint16s{
			values: values,
		}
		return vector, nil

	case zed.TypeUint32:
		values := make([]uint32, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, uint32(zed.DecodeUint(bs)))
			}
		}
		vector := &uint32s{
			values: values,
		}
		return vector, nil

	case zed.TypeUint64:
		values := make([]uint64, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				values = append(values, uint64(zed.DecodeUint(bs)))
			}
		}
		vector := &uint64s{
			values: values,
		}
		return vector, nil

	case zed.TypeNull:
		return &constants{}, nil

	case zed.TypeType:
		values := make([]zed.Type, 0, count)
		for _, segment := range segmap {
			err := readSegment(readerAt, buf, segment)
			if err != nil {
				return nil, err
			}
			it := zcode.Iter(*buf)
			for !it.Done() {
				bs := it.Next()
				typ, _ := zctx.DecodeTypeValue(bs)
				values = append(values, typ)
			}
		}
		vector := &types{
			values: values,
		}
		return vector, nil

	default:
		return nil, fmt.Errorf("unknown VNG type: %T", typ)
	}
}

func readSegment(readerAt io.ReaderAt, buf *[]byte, segment vngvector.Segment) error {
	*buf = slices.Grow((*buf)[:0], int(segment.MemLength))[:segment.MemLength]
	return segment.Read(readerAt, *buf)
}

func readSegmap(readerAt io.ReaderAt, segmap []vngvector.Segment) ([]byte, error) {
	var memLength int
	for _, segment := range segmap {
		memLength += int(segment.MemLength)
	}
	data := make([]byte, memLength)
	offset := 0
	for _, segment := range segmap {
		if err := segment.Read(readerAt, data[offset:offset+int(segment.MemLength)]); err != nil {
			return nil, err
		}
		offset += int(segment.MemLength)
	}
	return data, nil
}

func stripContainers(data []byte, countHint int) ([]byte, []int, error) {
	offsetFrom := 0
	offsetTo := 0
	offsets := make([]int, 0, countHint+1)
	offsets = append(offsets, offsetTo)
	for offsetFrom < len(data) {
		tag, tagLen := binary.Uvarint(data[offsetFrom:])
		if tagLen <= 0 || tag == 0 {
			return nil, nil, errBadTag
		}
		dataLen := int(tag - 1)
		// Shift data over to remove tag.
		// TODO Don't store tags in the VNG file in the first place.
		copy(data[offsetTo:offsetTo+dataLen], data[offsetFrom+tagLen:offsetFrom+tagLen+dataLen])
		offsetFrom += tagLen + dataLen
		offsetTo += dataLen
		offsets = append(offsets, offsetTo)
	}
	return data[:offsetTo], offsets, nil
}

// This must match exactly the effects of demand on `read`.
func typeAfterDemand(zctx *zed.Context, meta vngvector.Metadata, demandOut demand.Demand, typ zed.Type) zed.Type {
	if demand.IsNone(demandOut) {
		return zed.TypeNull
	}
	if demand.IsAll(demandOut) {
		return typ
	}
	switch meta := meta.(type) {
	case *vngvector.Nulls:
		return typeAfterDemand(zctx, meta.Values, demandOut, typ)

	case *vngvector.Record:
		typ := typ.(*zed.TypeRecord)
		var fields []zed.Field
		for i, fieldMeta := range meta.Fields {
			demandValueOut := demand.GetKey(demandOut, fieldMeta.Name)
			if !demand.IsNone(demandValueOut) {
				field := typ.Fields[i]
				fields = append(fields, zed.Field{
					Name: field.Name,
					Type: typeAfterDemand(zctx, fieldMeta.Values, demandValueOut, field.Type),
				})
			}
		}
		result, err := zctx.LookupTypeRecord(fields)
		if err != nil {
			// This should be unreachable - any subset of a valid type is also valid.
			panic(err)
		}
		return result

	case *vngvector.Union:
		typ := typ.(*zed.TypeUnion)
		types := make([]zed.Type, 0, len(typ.Types))
		for i, valueMeta := range meta.Values {
			types = append(types, typeAfterDemand(zctx, valueMeta, demandOut, typ.Types[i]))
		}
		return zctx.LookupTypeUnion(types)

	default:
		return typ
	}
}
