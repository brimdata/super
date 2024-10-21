package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Any interface {
	Type() super.Type
	Len() uint32
	Serialize(*zcode.Builder, uint32)
}

type Promotable interface {
	Any
	Promote(super.Type) Promotable
}

type Puller interface {
	Pull(done bool) (Any, error)
}

type Builder func(*zcode.Builder) bool
