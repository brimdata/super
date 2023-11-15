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
	reader := &reader{object.Zctx, object.ReaderAt, nil}
	tags, err := readInt64s(reader, object.Root)
	if err != nil {
		return nil, err
	}
	types := make([]zed.Type, len(object.Maps))
	values := make([]vector, len(object.Maps))
	for i, metadata := range object.Maps {
		types[i] = typeAfterDemand(object.Zctx, metadata, demandOut, metadata.Type(object.Zctx))
		value, err := read(reader, metadata, demandOut)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return &Vector{
		Context: object.Zctx,
		Types:   types,
		values:  values,
		tags:    tags,
	}, nil
}

type reader struct {
	zctx     *zed.Context
	readerAt io.ReaderAt
	buf      []byte
}

func read(reader *reader, meta vngvector.Metadata, demandOut demand.Demand) (vector, error) {
	if demand.IsNone(demandOut) {
		return &constants{}, nil
	}

	switch meta := meta.(type) {

	case *vngvector.Array:
		lengths, err := readInt64s(reader, meta.Lengths)
		if err != nil {
			return nil, err
		}
		elems, err := read(reader, meta.Values, demand.All())
		if err != nil {
			return nil, err
		}
		return &arrays{
			lengths: lengths,
			elems:   elems,
		}, nil

	case *vngvector.Const:
		return &constants{
			bytes: meta.Value.Bytes(),
		}, nil

	case *vngvector.Map:
		keys, err := read(reader, meta.Keys, demand.All())
		if err != nil {
			return nil, err
		}
		lengths, err := readInt64s(reader, meta.Lengths)
		if err != nil {
			return nil, err
		}
		values, err := read(reader, meta.Values, demand.All())
		if err != nil {
			return nil, err
		}
		return &maps{
			lengths: lengths,
			keys:    keys,
			values:  values,
		}, nil

	case *vngvector.Named:
		return read(reader, meta.Values, demandOut)

	case *vngvector.Nulls:
		runs, err := readInt64s(reader, meta.Runs)
		if err != nil {
			return nil, err
		}
		values, err := read(reader, meta.Values, demandOut)
		if err != nil {
			return nil, err
		}
		if len(runs) == 0 {
			return values, nil
		}
		return &nulls{
			runs:   runs,
			values: values,
		}, nil

	case *vngvector.Primitive:
		if len(meta.Dict) == 0 {
			return readPrimitive(reader, meta.Segmap, meta.Type(reader.zctx))
		}
		tags, err := readSegmap(reader.readerAt, meta.Segmap)
		if err != nil {
			return nil, err
		}
		return &dict{
			dict: meta.Dict,
			tags: tags,
		}, nil

	case *vngvector.Record:
		var fields []vector
		for _, fieldMeta := range meta.Fields {
			demandValueOut := demand.GetKey(demandOut, fieldMeta.Name)
			if !demand.IsNone(demandValueOut) {
				field, err := read(reader, fieldMeta.Values, demandValueOut)
				if err != nil {
					return nil, err
				}
				fields = append(fields, field)
			}
		}
		return &records{
			fields: fields,
		}, nil

	case *vngvector.Set:
		lengths, err := readInt64s(reader, meta.Lengths)
		if err != nil {
			return nil, err
		}
		elems, err := read(reader, meta.Values, demand.All())
		if err != nil {
			return nil, err
		}
		return &sets{
			lengths: lengths,
			elems:   elems,
		}, nil

	case *vngvector.Union:
		payloads := make([]vector, len(meta.Values))
		for i, valueMeta := range meta.Values {
			payload, err := read(reader, valueMeta, demandOut)
			if err != nil {
				return nil, err
			}
			payloads[i] = payload
		}
		tags, err := readInt64s(reader, meta.Tags)
		if err != nil {
			return nil, err
		}
		return &unions{
			payloads: payloads,
			tags:     tags,
		}, nil

	default:
		return nil, fmt.Errorf("unknown VNG meta type: %T", meta)
	}
}

func readInt64s(reader *reader, segmap []vngvector.Segment) ([]int64, error) {
	vector, err := readPrimitive(reader, segmap, zed.TypeInt64)
	if err != nil {
		return nil, err
	}
	return vector.(*int64s).values, nil
}

var errBadTag = errors.New("bad tag")

func readPrimitive(reader *reader, segmap []vngvector.Segment, typ zed.Type) (vector, error) {
	var count int
	for _, segment := range segmap {
		count += int(segment.Count)
	}

	switch typ {
	case zed.TypeBool:
		values := make([]bool, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeBool(it.Next()))
			}
		}
		return &bools{
			values: values,
		}, nil

	case zed.TypeBytes:
		data, err := readSegmap(reader.readerAt, segmap)
		if err != nil {
			return nil, err
		}
		data, offsets, err := stripContainers(data, count)
		if err != nil {
			return nil, err
		}
		return &byteses{
			data:    data,
			offsets: offsets,
		}, nil

	case zed.TypeDuration:
		values := make([]nano.Duration, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeDuration(it.Next()))
			}
		}
		return &durations{
			values: values,
		}, nil

	case zed.TypeFloat16:
		values := make([]float32, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeFloat16(it.Next()))
			}
		}
		return &float16s{
			values: values,
		}, nil

	case zed.TypeFloat32:
		values := make([]float32, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeFloat32(it.Next()))
			}
		}
		return &float32s{
			values: values,
		}, nil

	case zed.TypeFloat64:
		values := make([]float64, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeFloat64(it.Next()))
			}
		}
		return &float64s{
			values: values,
		}, nil

	case zed.TypeInt8:
		values := make([]int8, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, int8(zed.DecodeInt(it.Next())))
			}
		}
		return &int8s{
			values: values,
		}, nil

	case zed.TypeInt16:
		values := make([]int16, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, int16(zed.DecodeInt(it.Next())))
			}
		}
		return &int16s{
			values: values,
		}, nil

	case zed.TypeInt32:
		values := make([]int32, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, int32(zed.DecodeInt(it.Next())))
			}
		}
		return &int32s{
			values: values,
		}, nil

	case zed.TypeInt64:
		values := make([]int64, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, int64(zed.DecodeInt(it.Next())))
			}
		}
		return &int64s{
			values: values,
		}, nil

	case zed.TypeIP:
		values := make([]netip.Addr, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeIP(it.Next()))
			}
		}
		return &ips{
			values: values,
		}, nil

	case zed.TypeNet:
		values := make([]netip.Prefix, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeNet(it.Next()))
			}
		}
		return &nets{
			values: values,
		}, nil

	case zed.TypeString:
		data, err := readSegmap(reader.readerAt, segmap)
		if err != nil {
			return nil, err
		}
		data, offsets, err := stripContainers(data, count)
		if err != nil {
			return nil, err
		}
		return &strings{
			data:    data,
			offsets: offsets,
		}, nil

	case zed.TypeTime:
		values := make([]nano.Ts, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, zed.DecodeTime(it.Next()))
			}
		}
		return &times{
			values: values,
		}, nil

	case zed.TypeUint8:
		values := make([]uint8, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, uint8(zed.DecodeUint(it.Next())))
			}
		}
		return &uint8s{
			values: values,
		}, nil

	case zed.TypeUint16:
		values := make([]uint16, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, uint16(zed.DecodeUint(it.Next())))
			}
		}
		return &uint16s{
			values: values,
		}, nil

	case zed.TypeUint32:
		values := make([]uint32, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, uint32(zed.DecodeUint(it.Next())))
			}
		}
		return &uint32s{
			values: values,
		}, nil

	case zed.TypeUint64:
		values := make([]uint64, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				values = append(values, uint64(zed.DecodeUint(it.Next())))
			}
		}
		return &uint64s{
			values: values,
		}, nil

	case zed.TypeNull:
		return &constants{}, nil

	case zed.TypeType:
		values := make([]zed.Type, 0, count)
		for _, segment := range segmap {
			if err := readSegment(reader, segment); err != nil {
				return nil, err
			}
			for it := zcode.Iter(reader.buf); !it.Done(); {
				typ, _ := reader.zctx.DecodeTypeValue(it.Next())
				values = append(values, typ)
			}
		}
		return &types{
			values: values,
		}, nil

	default:
		return nil, fmt.Errorf("unknown VNG type: %T", typ)
	}
}

func readSegment(reader *reader, segment vngvector.Segment) error {
	reader.buf = slices.Grow((reader.buf)[:0], int(segment.MemLength))[:segment.MemLength]
	return segment.Read(reader.readerAt, reader.buf)
}

func readSegmap(readerAt io.ReaderAt, segmap []vngvector.Segment) ([]byte, error) {
	var memLength int
	for _, segment := range segmap {
		memLength += int(segment.MemLength)
	}
	data := make([]byte, memLength)
	offset := 0
	for _, segment := range segmap {
		if err := segment.Read(readerAt, data[offset:]); err != nil {
			return nil, err
		}
		offset += int(segment.MemLength)
	}
	return data, nil
}

func stripContainers(data []byte, countHint int) ([]byte, []int, error) {
	var offsetFrom, offsetTo int
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

	case *vngvector.Named:
		return typeAfterDemand(zctx, meta.Values, demandOut, typ.(*zed.TypeNamed).Type)

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
