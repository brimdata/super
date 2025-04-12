package vector

import (
	"net/netip"

	"github.com/brimdata/super/vector/bitvec"
)

type (
	ArrayLoader interface {
		Load() (*Array, bitvec.Bits)
	}
	BytesLoader interface {
		Load() (BytesTable, bitvec.Bits)
	}
	BitsLoader interface {
		Load() (bitvec.Bits, bitvec.Bits)
	}
	DictLoader interface {
		Load() ([]byte, []uint32, bitvec.Bits)
	}
	FloatLoader interface {
		Load() ([]float64, bitvec.Bits)
	}
	IntLoader interface {
		Load() ([]int64, bitvec.Bits)
	}
	IPLoader interface {
		Load() ([]netip.Addr, bitvec.Bits)
	}
	NetLoader interface {
		Load() ([]netip.Prefix, bitvec.Bits)
	}
	NullsLoader interface {
		Load() bitvec.Bits
	}
	PrimitiveLoader interface {
		Load() (any, bitvec.Bits)
	}
	UintLoader interface {
		Load() ([]uint64, bitvec.Bits)
	}
	Uint32Loader interface {
		Load() ([]uint32, bitvec.Bits)
	}
)
