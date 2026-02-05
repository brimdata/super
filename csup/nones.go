package csup

import (
	"io"

	"golang.org/x/sync/errgroup"
)

// NonesEncoder emits a sequence of runs of the length of alternating sequences
// of nones and values, beginning with nones.  Every run is non-zero except for
// the first, which may be zero when the first value is non-none.
type NonesEncoder struct {
	runs  Uint32Encoder
	run   uint32
	none  bool
	count uint32
}

/*
func (n *NonesEncoder) Write(body scode.Bytes) {
	if body != nil {
		n.touchValue()
		n.values.Write(body)
	} else {
		n.touchNull()
	}
}
*/

func (n *NonesEncoder) touchValue() {
	if !n.none {
		n.run++
	} else {
		n.runs.Write(n.run)
		n.run = 1
		n.none = false
	}
}

func (n *NonesEncoder) touchNone() {
	n.count++
	if n.none {
		n.run++
	} else {
		n.runs.Write(n.run)
		n.run = 1
		n.none = true
	}
}

func (n *NonesEncoder) Encode(group *errgroup.Group) {
	//n.values.Encode(group)
	if n.count != 0 {
		if n.run > 0 {
			n.runs.Write(n.run)
		}
		n.runs.Encode(group)
	}
}

/* do this in Field
func (n *NonesEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, values := n.values.Metadata(cctx, off)
	if n.count == 0 {
		return off, values
	}
	off, runs := n.runs.Segment(off)
	return off, cctx.enter(&Nulls{
		Runs:   runs,
		Values: values,
		Count:  n.count,
	})
}
*/

func (n *NonesEncoder) Emit(w io.Writer) error {
	//if err := n.values.Emit(w); err != nil {
	//	return err
	//}
	if n.count != 0 {
		return n.runs.Emit(w)
	}
	return nil
}
