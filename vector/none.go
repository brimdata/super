package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

// A None vector arises from values not present in option types.
// In general, nones outside of options types should be handled by the query
// and ultimately turned into errors and never serialized.  However, it
// is possible to serialize them and inspect them when necessary for debugging etc.
type None struct {
	len uint32
}

func NewNone(len uint32) *None {
	return &None{len}
}

func (*None) Kind() Kind {
	return KindNone
}

func (n *None) Len() uint32 {
	return n.len
}

func (*None) Serialize(b *scode.Builder, _ uint32) {
	b.Append(nil)
}

func (*None) Type() super.Type {
	return super.TypeNone
}
