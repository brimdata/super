package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Any interface {
	Type() super.Type
	Len() uint32
	Serialize(*scode.Builder, uint32)
}

type Promotable interface {
	Any
	Promote(super.Type) Promotable
}

type Puller interface {
	Pull(done bool) (Any, error)
}
