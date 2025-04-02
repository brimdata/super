package csup

import (
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type DictEncoder struct {
	typ    super.Type
	tags   map[string]byte
	index  []byte
	counts *Uint32Encoder
	values Encoder
}

func NewDictEncoder(typ super.Type, values Encoder) *DictEncoder {
	if _, ok := values.(resetter); !ok {
		panic("Dict values encoder must be resettable")
	}
	var tags map[string]byte
	if id := typ.ID(); id != super.IDUint8 && id != super.IDInt8 && id != super.IDBool {
		// Don't bother using a dictionary (which takes 8-bit tags) to encode
		// other 8-bit values.
		tags = make(map[string]byte)
	}
	return &DictEncoder{
		typ:    typ,
		tags:   tags,
		values: values,
	}
}

func (d *DictEncoder) Write(body zcode.Bytes) {
	d.values.Write(body)
	if d.tags != nil {
		tag, ok := d.tags[string(body)]
		if !ok {
			len := len(d.tags)
			if len > math.MaxUint8 {
				d.tags = nil
				return
			}
			tag = byte(len)
			d.tags[string(body)] = tag
		}
		d.index = append(d.index, tag)
	}
}

func (d *DictEncoder) Const() *Const {
	if len(d.tags) != 1 {
		return nil
	}
	var bytes zcode.Bytes
	for b := range d.tags {
		bytes = []byte(b)
	}
	return &Const{
		Value: super.NewValue(d.typ, bytes),
		Count: uint32(len(d.index)),
	}
}

func (d *DictEncoder) isValid() bool {
	return len(d.tags) >= 1 && len(d.index) > len(d.tags)
}

func (d *DictEncoder) Encode(group *errgroup.Group) {
	if !d.isValid() {
		d.values.Encode(group)
		return
	}
	// If len == 1 then this is a const so do not encode anything.
	if len(d.tags) == 1 {
		return
	}
	d.encodeValues(group)
	d.encodeCounts(group)
}

func (d *DictEncoder) encodeValues(group *errgroup.Group) {
	byteSlices := make([]zcode.Bytes, len(d.tags))
	for key, tag := range d.tags {
		byteSlices[tag] = zcode.Bytes(key)
	}
	d.values.(resetter).reset()
	for _, v := range byteSlices {
		d.values.Write(v)
	}
	d.values.Encode(group)
}

func (d *DictEncoder) encodeCounts(group *errgroup.Group) {
	counts := make([]uint32, len(d.tags))
	for _, v := range d.index {
		counts[v]++
	}
	d.counts = &Uint32Encoder{vals: counts}
	d.counts.Encode(group)
}

func (d *DictEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	if !d.isValid() {
		return d.values.Metadata(cctx, off)
	}
	if c := d.Const(); c != nil {
		return off, cctx.enter(c)
	}
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
	if !d.isValid() {
		return d.values.Emit(w)
	}
	if len(d.tags) == 1 {
		return nil
	}
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
