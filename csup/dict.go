package csup

import (
	"io"

	"github.com/brimdata/super"
	"golang.org/x/sync/errgroup"
)

type DictEncoder struct {
	PrimitiveEncoder
	typ super.Type

	// These fields are derived after Encode is called.
	counts *Uint32Encoder
	index  []byte
	const_ *Const
}

func NewDictEncoder(typ super.Type, values PrimitiveEncoder) Encoder {
	if id := typ.ID(); id == super.IDUint8 || id == super.IDInt8 || id == super.IDBool {
		return values
	}
	return &DictEncoder{
		typ:              typ,
		PrimitiveEncoder: values,
	}
}

func (d *DictEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		entries, index, counts := d.Dict()
		if entries == nil {
			d.PrimitiveEncoder.Encode(group)
			return nil
		}
		if len(counts) == 1 {
			d.const_ = &Const{
				Value: d.ConstValue(),
				Count: uint32(len(index)),
			}
			return nil
		}
		if !isValidDict(len(index), len(counts)) {
			d.PrimitiveEncoder.Encode(group)
			return nil
		}
		d.index = index
		d.PrimitiveEncoder = entries
		d.counts = &Uint32Encoder{vals: counts}
		d.PrimitiveEncoder.Encode(group)
		d.counts.Encode(group)
		return nil
	})
}

func isValidDict(len, card int) bool {
	return card >= 1 && card < len
}

func (d *DictEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	if d.const_ != nil {
		return off, cctx.enter(d.const_)
	}
	if d.counts == nil {
		return d.PrimitiveEncoder.Metadata(cctx, off)
	}
	meta := &Dict{Length: uint32(len(d.index))}
	off, meta.Values = d.PrimitiveEncoder.Metadata(cctx, off)
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
	if d.const_ != nil {
		return nil
	}
	if err := d.PrimitiveEncoder.Emit(w); err != nil {
		return err
	}
	if d.counts == nil {
		return nil
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
