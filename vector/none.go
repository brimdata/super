package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type None struct {
	*Error
}

func NewNone(sctx *super.Context, len uint32) *None {
	return &None{Error: NewMissing(sctx, len)}
}

func (n *None) Derive(len uint32) *None {
	return &None{&Error{Typ: n.Typ, Vals: NewConstString("missing", len)}}
}

func (*None) Kind() Kind {
	return KindNone
}

func (*None) Serialize(b *scode.Builder, _ uint32) {
	b.Append(nil)
}

func (*None) Type() super.Type {
	return super.TypeNone
}
