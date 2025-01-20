package vng

import (
	"io"

	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

// NullsEncoder emits a sequence of runs of the length of alternating sequences
// of nulls and values, beginning with nulls.  Every run is non-zero except for
// the first, which may be zero when the first value is non-null.
type NullsEncoder struct {
	values Encoder
	runs   Uint32Encoder
	run    uint32
	null   bool
	count  uint32
}

func NewNullsEncoder(values Encoder) *NullsEncoder {
	return &NullsEncoder{
		values: values,
	}
}

func (n *NullsEncoder) Write(body zcode.Bytes) {
	if body != nil {
		n.touchValue()
		n.values.Write(body)
	} else {
		n.touchNull()
	}
}

func (n *NullsEncoder) touchValue() {
	if !n.null {
		n.run++
	} else {
		n.runs.Write(n.run)
		n.run = 1
		n.null = false
	}
}

func (n *NullsEncoder) touchNull() {
	n.count++
	if n.null {
		n.run++
	} else {
		n.runs.Write(n.run)
		n.run = 1
		n.null = true
	}
}

func (n *NullsEncoder) Encode(group *errgroup.Group) {
	n.values.Encode(group)
	if n.count != 0 {
		if n.run > 0 {
			n.runs.Write(n.run)
		}
		n.runs.Encode(group)
	}
}

func (n *NullsEncoder) Metadata(off uint64) (uint64, Metadata) {
	off, values := n.values.Metadata(off)
	if n.count == 0 {
		return off, values
	}
	off, runs := n.runs.Segment(off)
	return off, &Nulls{
		Runs:   runs,
		Values: values,
		Count:  n.count,
	}
}

func (n *NullsEncoder) Emit(w io.Writer) error {
	if err := n.values.Emit(w); err != nil {
		return err
	}
	if n.count != 0 {
		return n.runs.Emit(w)
	}
	return nil
}
