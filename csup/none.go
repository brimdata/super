package csup

import (
	"io"

	"golang.org/x/sync/errgroup"
)

type NoneEncoder struct {
	len uint32
}

func NewNoneEncoder(len uint32) *NoneEncoder {
	return &NoneEncoder{len}
}

func (n *NoneEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	return off, cctx.enter(&None{n.len})
}

func (*NoneEncoder) Encode(group *errgroup.Group) {}
func (*NoneEncoder) Emit(io.Writer) error         { return nil }
