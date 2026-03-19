package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type DynamicEncoder struct {
	cctx   *Context
	tags   []uint32
	tagEnc *Uint32Encoder
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
func (d *DynamicEncoder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	if dynamic, ok := vec.(*vector.Dynamic); ok {
		d.appendDynamic(dynamic)
	} else {
		d.appendVec(vec)
	}
}

func (d *DynamicEncoder) appendDynamic(vec *vector.Dynamic) {
	var tagmap []uint32 // input tags to local tags
	for _, vec := range vec.Values {
		tagmap = append(tagmap, d.lookupType(vec.Type()))
	}
	for _, intag := range vec.Tags {
		d.tags = append(d.tags, tagmap[intag])
	}
	for intag, vec := range vec.Values {
		d.values[tagmap[intag]].Write(vec)
	}
	d.len += vec.Len()
}

func (d *DynamicEncoder) appendVec(vec vector.Any) {
	tag := d.lookupType(vec.Type())
	for range vec.Len() {
		//XXX there's a better way, but let's get this working
		d.tags = append(d.tags, tag)
	}
	d.values[tag].Write(vec)
	d.len += vec.Len()
}

func (d *DynamicEncoder) lookupType(typ super.Type) uint32 {
	tag, ok := d.which[typ]
	if !ok {
		tag = uint32(len(d.values))
		d.values = append(d.values, NewEncoder(d.cctx, typ))
		d.which[typ] = tag
	}
	return tag
}

func (d *DynamicEncoder) Encode() (ID, uint64, error) {
	var group errgroup.Group
	d.tagEnc = &Uint32Encoder{vals: d.tags}
	if len(d.values) > 1 {
		d.tagEnc.Encode(&group)
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
	off, tags := d.tagEnc.Segment(0)
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
		if err := d.tagEnc.Emit(w); err != nil {
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
