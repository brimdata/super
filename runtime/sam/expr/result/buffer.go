package result

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/zcode"
)

type Buffer zcode.Bytes

func (b *Buffer) Int(v int64) zcode.Bytes {
	*b = Buffer(super.AppendInt(zcode.Bytes((*b)[:0]), v))
	return zcode.Bytes(*b)
}

func (b *Buffer) Uint(v uint64) zcode.Bytes {
	*b = Buffer(super.AppendUint(zcode.Bytes((*b)[:0]), v))
	return zcode.Bytes(*b)
}

func (b *Buffer) Float32(v float32) zcode.Bytes {
	*b = Buffer(super.AppendFloat32(zcode.Bytes((*b)[:0]), v))
	return zcode.Bytes(*b)
}

func (b *Buffer) Float64(v float64) zcode.Bytes {
	*b = Buffer(super.AppendFloat64(zcode.Bytes((*b)[:0]), v))
	return zcode.Bytes(*b)
}

func (b *Buffer) Time(v nano.Ts) zcode.Bytes {
	*b = Buffer(super.AppendTime(zcode.Bytes((*b)[:0]), v))
	return zcode.Bytes(*b)
}
