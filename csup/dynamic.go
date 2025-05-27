package csup

import (
	"bytes"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/bsupio"
	"golang.org/x/sync/errgroup"
)

type DynamicEncoder struct {
	cctx   *Context
	tags   Uint32Encoder
	values []Encoder
	which  map[super.Type]uint32
	len    uint32

	bsupBuf    *bytes.Buffer
	bsupWriter *bsupio.Writer
}

func NewDynamicEncoder() *DynamicEncoder {
	var buf bytes.Buffer
	return &DynamicEncoder{
		cctx:       NewContext(),
		which:      make(map[super.Type]uint32),
		bsupBuf:    &buf,
		bsupWriter: bsupio.NewWriter(zio.NopCloser(&buf)),
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
		d.values = append(d.values, newBSUPEncoder(typ, d.bsupWriter))
		d.which[typ] = tag
	}
	d.tags.Write(tag)
	d.len++
	d.values[tag].Write(val.Bytes())
}

func (d *DynamicEncoder) Encode() (ID, uint64, uint64, error) {
	var group errgroup.Group
	if len(d.values) > 1 {
		d.tags.Encode(&group)
	}
	for _, val := range d.values {
		val.Encode(&group)
	}
	group.Go(d.bsupWriter.Close)
	if err := group.Wait(); err != nil {
		return 0, 0, 0, err
	}
	if bytes.Equal(d.bsupBuf.Bytes(), []byte{bsupio.EOS}) {
		// No values so reset to empty.
		d.bsupBuf.Reset()
	}
	bsupSize := uint64(d.bsupBuf.Len())
	if len(d.values) == 1 {
		off, id := d.values[0].Metadata(d.cctx, 0)
		return id, off, bsupSize, nil
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
	}), off, bsupSize, nil
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
	_, err := w.Write(d.bsupBuf.Bytes())
	return err
}
