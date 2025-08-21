package result

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/scode"
)

type Buffer scode.Bytes

func (b *Buffer) Int(v int64) scode.Bytes {
	*b = Buffer(super.AppendInt(scode.Bytes((*b)[:0]), v))
	return scode.Bytes(*b)
}

func (b *Buffer) Uint(v uint64) scode.Bytes {
	*b = Buffer(super.AppendUint(scode.Bytes((*b)[:0]), v))
	return scode.Bytes(*b)
}

func (b *Buffer) Float32(v float32) scode.Bytes {
	*b = Buffer(super.AppendFloat32(scode.Bytes((*b)[:0]), v))
	return scode.Bytes(*b)
}

func (b *Buffer) Float64(v float64) scode.Bytes {
	*b = Buffer(super.AppendFloat64(scode.Bytes((*b)[:0]), v))
	return scode.Bytes(*b)
}

func (b *Buffer) Time(v nano.Ts) scode.Bytes {
	*b = Buffer(super.AppendTime(scode.Bytes((*b)[:0]), v))
	return scode.Bytes(*b)
}
