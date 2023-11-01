package vector

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zio"
)

type Materializer struct {
	vector        *Vector
	materializers []materializer
	index         int
	builder       zcode.Builder
	value         zed.Value
}

func (v *Vector) NewMaterializer() Materializer {
	materializers := make([]materializer, len(v.Types))
	for i, value := range v.values {
		materializers[i] = value.newMaterializer()
	}
	return Materializer{
		vector:        v,
		materializers: materializers,
	}
}

var _ zio.Reader = (*Materializer)(nil)

func (m *Materializer) Read() (*zed.Value, error) {
	if m.index >= len(m.vector.tags) {
		return nil, nil
	}
	tag := m.vector.tags[m.index]
	typ := m.vector.Types[tag]
	m.builder.Truncate()
	m.materializers[tag](&m.builder)
	m.value = *zed.NewValue(typ, m.builder.Bytes().Body())
	m.index++
	return &m.value, nil
}

// TODO This exists as a builtin in go 1.21
func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

type materializer func(*zcode.Builder)

func (v *bools) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeBool(v.values[index]))
		index++
	}
}

func (v *byteses) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeBytes(v.values[index]))
		index++
	}
}

func (v *durations) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeDuration(v.values[index]))
		index++
	}
}

func (v *float16s) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeFloat16(v.values[index]))
		index++
	}
}

func (v *float32s) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeFloat32(v.values[index]))
		index++
	}
}

func (v *float64s) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeFloat64(v.values[index]))
		index++
	}
}

func (v *ints) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeInt(v.values[index]))
		index++
	}
}

func (v *ips) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeIP(v.values[index]))
		index++
	}
}

func (v *nets) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeNet(v.values[index]))
		index++
	}
}

func (v *strings) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeString(v.values[index]))
		index++
	}
}

func (v *types) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeTypeValue(v.values[index]))
		index++
	}
}

func (v *times) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeTime(v.values[index]))
		index++
	}
}

func (v *uints) newMaterializer() materializer {
	var index int
	return func(builder *zcode.Builder) {
		builder.Append(zed.EncodeUint(v.values[index]))
		index++
	}
}

func (v *arrays) newMaterializer() materializer {
	var index int
	elemMaterializer := v.elems.newMaterializer()
	return func(builder *zcode.Builder) {
		length := int(v.lengths[index])
		builder.BeginContainer()
		for i := 0; i < length; i++ {
			elemMaterializer(builder)
		}
		builder.EndContainer()
		index++
	}
}

func (v *constants) newMaterializer() materializer {
	bytes := v.value.Bytes()
	return func(builder *zcode.Builder) {
		builder.Append(bytes)
	}
}

func (v *maps) newMaterializer() materializer {
	var index int
	keyMaterializer := v.keys.newMaterializer()
	valueMaterializer := v.values.newMaterializer()
	return func(builder *zcode.Builder) {
		length := int(v.lengths[index])
		builder.BeginContainer()
		for i := 0; i < length; i++ {
			keyMaterializer(builder)
			valueMaterializer(builder)
		}
		builder.TransformContainer(zed.NormalizeMap)
		builder.EndContainer()
		index++
	}
}

func (v *nulls) newMaterializer() materializer {
	var index int
	valueMaterializer := v.values.newMaterializer()
	return func(builder *zcode.Builder) {
		if v.mask.ContainsInt(index) {
			valueMaterializer(builder)
		} else {
			builder.Append(nil)
		}
		index++
	}
}

func (v *records) newMaterializer() materializer {
	fieldMaterializers := make([]materializer, len(v.fields))
	for i, field := range v.fields {
		fieldMaterializers[i] = field.newMaterializer()
	}
	return func(builder *zcode.Builder) {
		builder.BeginContainer()
		for _, fieldMaterializer := range fieldMaterializers {
			fieldMaterializer(builder)
		}
		builder.EndContainer()
	}
}

func (v *unions) newMaterializer() materializer {
	var index int
	payloadMaterializers := make([]materializer, len(v.payloads))
	for i, payload := range v.payloads {
		payloadMaterializers[i] = payload.newMaterializer()
	}
	return func(builder *zcode.Builder) {
		builder.BeginContainer()
		tag := v.tags[index]
		builder.Append(zed.EncodeInt(tag))
		payloadMaterializers[tag](builder)
		builder.EndContainer()
		index++
	}
}
