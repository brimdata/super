package vector

import "github.com/brimdata/super/vector/bitvec"

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
	MapLoader interface {
		Load() (*Map, bitvec.Bits)
	}
	NullsLoader interface {
		Load() bitvec.Bits
	}
	RecordLoader interface {
		Load() ([]Any, bitvec.Bits)
	}
	IntLoader interface {
		Load() ([]int64, bitvec.Bits)
	}
	UintLoader interface {
		Load() ([]uint64, bitvec.Bits)
	}
	Uint32Loader interface {
		Load() ([]uint32, bitvec.Bits)
	}
)
