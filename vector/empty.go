package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

// An Empty vector represents a vector with a type of zero length.
// These are needed to support recursive types since the invariant on union
// values is that the Type method may be called on a zero-length element of
// a union but zero-length values of recursel XXX
type Empty struct {
	typ super.Type
}

var _ Any = (*Empty)(nil)

func NewEmpty(typ super.Type) *Empty {
	return &Empty{typ}
}

func (e *Empty) Kind() Kind {
	panic(e)
}

func (e *Empty) Type() super.Type {
	return e.typ
}

func (*Empty) Len() uint32 {
	return 0
}

func (*Empty) Serialize(*scode.Builder, uint32) {}
