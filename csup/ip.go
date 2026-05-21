package csup

import (
	"io"
	"net/netip"

	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type IPEncoder struct {
	vals     []netip.Addr
	table    *BytesTableEncoder
	min, max netip.Addr
}

func NewIPEncoder(vals []netip.Addr) *IPEncoder {
	return &IPEncoder{
		vals: vals,
	}
}

func (i *IPEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		var bytes []byte
		offsets := []uint32{0}
		for _, ip := range i.vals {
			var err error
			if bytes, err = ip.AppendBinary(bytes); err != nil {
				panic(err)
			}
			offsets = append(offsets, uint32(len(bytes)))
		}
		i.table = NewBytesTableEncoder(vector.NewBytesTable(offsets, bytes))
		i.table.Encode(group)
		return nil
	})
	if len(i.vals) > 0 {
		group.Go(func() error {
			i.min, i.max = i.vals[0], i.vals[0]
			for _, v := range i.vals {
				if v.Compare(i.min) < 0 {
					i.min = v
				}
				if v.Compare(i.max) > 0 {
					i.max = v
				}
			}
			return nil
		})
	}
}

func (i *IPEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, bytesLoc, offsLoc := i.table.Segment(off)
	return off, cctx.enter(&IP{
		Bytes:   bytesLoc,
		Offsets: offsLoc,
		Min:     i.min,
		Max:     i.max,
		Count:   uint32(len(i.vals)),
	})
}

func (i *IPEncoder) Emit(w io.Writer) error {
	return i.table.Emit(w)
}
