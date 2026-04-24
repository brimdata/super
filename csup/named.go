package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type NamedEncoder struct {
	encoder Encoder
	cctx    *Context
	typ     *super.TypeNamed
}

func (n *NamedEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	if n.encoder == nil {
		return off, cctx.enter(&Empty{Type: n.typ})
	}
	off, id := n.encoder.Metadata(cctx, off)
	return off, cctx.enter(&Named{n.typ.Name, id})
}

func (n *NamedEncoder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	if n.encoder == nil {
		n.encoder = NewEncoder(n.cctx, n.typ.Type)
	}
	n.encoder.Write(vec.(*vector.Named).Any)
}

func (n *NamedEncoder) Encode(group *errgroup.Group) {
	if n.encoder != nil {
		n.encoder.Encode(group)
	}
}

func (n *NamedEncoder) Emit(w io.Writer) error {
	if n.encoder != nil {
		return n.encoder.Emit(w)
	}
	return nil
}
