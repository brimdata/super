package vng

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zio"
	"golang.org/x/sync/errgroup"
)

type DynamicEncoder struct {
	tags   *Int64Encoder
	values []Encoder
	which  map[super.Type]int
	len    uint32
}

var _ zio.Writer = (*DynamicEncoder)(nil)

func NewDynamicEncoder() *DynamicEncoder {
	return &DynamicEncoder{
		tags:  NewInt64Encoder(),
		which: make(map[super.Type]int),
	}
}

// The dynamic encoder self-organizes around the types that are
// written to it.  No need to define the schema up front!
// We track the types seen first-come, first-served and the
// VNG metadata structure follows accordingly.
func (d *DynamicEncoder) Write(val super.Value) error {
	typ := val.Type()
	tag, ok := d.which[typ]
	if !ok {
		tag = len(d.values)
		d.values = append(d.values, NewEncoder(typ))
		d.which[typ] = tag
	}
	d.tags.Write(int64(tag))
	d.len++
	d.values[tag].Write(val.Bytes())
	return nil
}

func (d *DynamicEncoder) Encode() (Metadata, uint64, error) {
	var group errgroup.Group
	if len(d.values) > 1 {
		d.tags.Encode(&group)
	}
	for _, val := range d.values {
		val.Encode(&group)
	}
	if err := group.Wait(); err != nil {
		return nil, 0, err
	}
	if len(d.values) == 1 {
		off, meta := d.values[0].Metadata(0)
		return meta, off, nil
	}
	values := make([]Metadata, 0, len(d.values))
	off, tags := d.tags.Metadata(0)
	for _, val := range d.values {
		var meta Metadata
		off, meta = val.Metadata(off)
		values = append(values, meta)
	}
	return &Dynamic{
		Tags:   tags.(*Primitive).Location,
		Values: values,
		Length: d.len,
	}, off, nil
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

type dynamicBuilder struct {
	types   []super.Type
	tags    *Int64Decoder
	values  []Builder
	builder *zcode.Builder
}

func newDynamicBuilder(zctx *super.Context, d *Dynamic, reader io.ReaderAt) (*dynamicBuilder, error) {
	values := make([]Builder, 0, len(d.Values))
	types := make([]super.Type, 0, len(d.Values))
	for _, val := range d.Values {
		r, err := NewBuilder(val, reader)
		if err != nil {
			return nil, err
		}
		values = append(values, r)
		types = append(types, val.Type(zctx))
	}
	return &dynamicBuilder{
		types:   types,
		tags:    NewInt64Decoder(d.Tags, reader),
		values:  values,
		builder: zcode.NewBuilder(),
	}, nil
}

func (d *dynamicBuilder) Read() (*super.Value, error) {
	b := d.builder
	b.Truncate()
	tag, err := d.tags.Next()
	if err != nil {
		if err == io.EOF {
			err = nil
		}
		return nil, err
	}
	if int(tag) >= len(d.types) {
		return nil, fmt.Errorf("bad tag encountered scanning VNG dynamic: tag %d when only %d types", tag, len(d.types))
	}
	if err := d.values[tag].Build(b); err != nil {
		return nil, err
	}
	return super.NewValue(d.types[tag], b.Bytes().Body()).Ptr(), nil
}

func NewZedReader(zctx *super.Context, meta Metadata, r io.ReaderAt) (zio.Reader, error) {
	if d, ok := meta.(*Dynamic); ok {
		return newDynamicBuilder(zctx, d, r)
	}
	values, err := NewBuilder(meta, r)
	if err != nil {
		return nil, err
	}
	return &vectorBuilder{
		typ:     meta.Type(zctx),
		values:  values,
		builder: zcode.NewBuilder(),
		count:   meta.Len(),
	}, nil
}

type vectorBuilder struct {
	typ     super.Type
	values  Builder
	builder *zcode.Builder
	count   uint32
}

func (v *vectorBuilder) Read() (*super.Value, error) {
	if v.count == 0 {
		return nil, nil
	}
	v.count--
	b := v.builder
	b.Truncate()
	if err := v.values.Build(b); err != nil {
		return nil, err
	}
	return super.NewValue(v.typ, b.Bytes().Body()).Ptr(), nil
}
