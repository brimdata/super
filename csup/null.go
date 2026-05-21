package csup

import (
	"io"

	"golang.org/x/sync/errgroup"
)

type NullEncoder struct {
	len uint32
}

func NewNullEncoder(len uint32) *NullEncoder {
	return &NullEncoder{len}
}

func (n *NullEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	return off, cctx.enter(&Null{n.len})
}

func (*NullEncoder) Encode(*errgroup.Group) {}
func (*NullEncoder) Emit(io.Writer) error   { return nil }
