package csup

import (
	"io"

	"github.com/brimdata/super"
	"golang.org/x/sync/errgroup"
)

type DynamicEncoder struct {
	cctx   *Context
	tags   Uint32Encoder
	values []Encoder
	which  map[super.Type]uint32
	len    uint32
}

func NewDynamicEncoder() *DynamicEncoder {
	return &DynamicEncoder{
		cctx:  NewContext(),
		which: make(map[super.Type]uint32),
	}
}

// The dynamic encoder self-organizes around the types that are
// written to it.  No need to define the schema up front!
// We track the types seen first-come, first-served and the
// CSUP metadata structure follows accordingly.
func (d *DynamicEncoder) Write(val super.Value) {
	typ := val.Type()
	tag, ok := d.which[typ]
	if !ok {
		tag = uint32(len(d.values))
		d.values = append(d.values, NewEncoder(typ))
		d.which[typ] = tag
	}
	d.tags.Write(tag)
	d.len++
	d.values[tag].Write(val.Bytes())
}

func (d *DynamicEncoder) Encode() (ID, uint64, error) {
	var group errgroup.Group
	if len(d.values) > 1 {
		d.tags.Encode(&group)
	}
	for _, val := range d.values {
		val.Encode(&group)
	}
	if err := group.Wait(); err != nil {
		return 0, 0, err
	}
	if len(d.values) == 1 {
		off, id := d.values[0].Metadata(d.cctx, 0)
		return id, off, nil
	}
	values := make([]ID, 0, len(d.values))
	off, tags := d.tags.Segment(0)
	for _, val := range d.values {
		var id ID
		off, id = val.Metadata(d.cctx, off)
		values = append(values, id)
	}
	return d.cctx.enter(&Dynamic{
		Tags:   tags,
		Values: values,
		Length: d.len,
	}), off, nil
}

func (d *DynamicEncoder) Emit(w io.Writer) error {
	if len(d.values) > 1 {
		if err := d.tags.Emit(w); err != nil {
			return err
		}
	}
	for _, value := range d.values {
		if err := value.Emit(w); err != nil {
			return err
		}
	}
	return nil
}
