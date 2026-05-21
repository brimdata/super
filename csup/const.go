package csup

import (
	"io"

	"github.com/brimdata/super"
	"golang.org/x/sync/errgroup"
)

type ConstEncoder struct {
	val super.Value
	len uint32
}

func (*ConstEncoder) Encode(group *errgroup.Group) {}

func (c *ConstEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	return off, cctx.enter(&Const{c.val, c.len})
}

func (*ConstEncoder) Emit(w io.Writer) error { return nil }
