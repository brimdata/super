package csup

import (
	"io"

	"golang.org/x/sync/errgroup"
)

type DictEncoder struct {
	values Encoder
	counts *Uint32Encoder
	index  []byte
}

func (d *DictEncoder) Encode(group *errgroup.Group) {
	d.values.Encode(group)
	d.counts.Encode(group)
}

func (d *DictEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	meta := &Dict{Length: uint32(len(d.index))}
	off, meta.Values = d.values.Metadata(cctx, off)
	off, meta.Counts = d.counts.Segment(off)
	len := uint64(len(d.index))
	meta.Index = Segment{
		Offset:    off,
		Length:    len,
		MemLength: len,
	}
	return off + len, cctx.enter(meta)
}

func (d *DictEncoder) Emit(w io.Writer) error {
	if err := d.values.Emit(w); err != nil {
		return err
	}
	if err := d.counts.Emit(w); err != nil {
		return err
	}
	var err error
	if len(d.index) > 0 {
		_, err = w.Write(d.index)
	}
	return err
}
