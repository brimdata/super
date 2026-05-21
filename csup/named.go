package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type NamedEncoder struct {
	encoder Encoder
	typ     *super.TypeNamed
}

func NewNamedEncoder(cctx *Context, named *vector.Named) Encoder {
	e := &NamedEncoder{typ: named.Typ}
	if _, ok := named.Any.(*vector.Empty); !ok {
		e.encoder = NewEncoder(cctx, named.Any)
	}
	return e
}

func (n *NamedEncoder) Encode(group *errgroup.Group) {
	if n.encoder != nil {
		n.encoder.Encode(group)
	}
}

func (n *NamedEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	if n.encoder == nil {
		return off, cctx.enter(&Empty{Type: n.typ})
	}
	off, id := n.encoder.Metadata(cctx, off)
	return off, cctx.enter(&Named{n.typ.Name, id})
}

func (n *NamedEncoder) Emit(w io.Writer) error {
	if n.encoder != nil {
		return n.encoder.Emit(w)
	}
	return nil
}
