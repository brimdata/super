package bsup

import (
	"errors"
	"fmt"
	"io"

	"github.com/superdb/super"
	"github.com/superdb/super/pkg/field"
	"github.com/superdb/super/scode"
)

type Object struct {
	cctx     *Context
	readerAt io.ReaderAt
	header   DataHeader
}

func NewObject(r io.ReaderAt) (*Object, error) {
	s, err := ReadSection(r)
	if err != nil {
		return nil, err
	}
	if s.Type != SectionObject {
		return nil, errors.New("cannot create object from footer section")
	}
	return NewObjectFromHeader(r, s.Object)
}

func NewObjectFromHeader(r io.ReaderAt, hdr DataHeader) (*Object, error) {
	cctx := NewContext()
	off := int64(HeaderSize + DataHeaderSize)
	if err := cctx.readMeta(io.NewSectionReader(r, off, int64(hdr.MetaSize))); err != nil {
		return nil, err
	}
	if hdr.Root >= uint32(len(cctx.values)) {
		return nil, fmt.Errorf("BSUP root ID %d larger than values table (len %d)", hdr.Root, len(cctx.values))
	}
	cctx.subtypesReader = io.NewSectionReader(r, off+int64(hdr.MetaSize), int64(hdr.TypeSize))
	return &Object{
		cctx:     cctx,
		readerAt: io.NewSectionReader(r, off+int64(hdr.MetaSize+hdr.TypeSize), int64(hdr.DataSize)),
		header:   hdr,
	}, nil
}

func (o *Object) Close() error {
	if closer, ok := o.readerAt.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (o *Object) Context() *Context {
	return o.cctx
}

func (o *Object) Root() ID {
	return ID(o.header.Root)
}

func (o *Object) DataReader() io.ReaderAt {
	return o.readerAt
}

func (o *Object) Size() uint64 {
	return HeaderSize + o.header.Size()
}

func (o *Object) ProjectMetadata(sctx *super.Context, projection field.Projection) []super.Value {
	var b scode.Builder
	var values []super.Value
	root := o.cctx.Lookup(o.Root())
	if root, ok := root.(*Dynamic); ok {
		for _, id := range root.Values {
			b.Reset()
			typ := metadataValue(o.cctx, sctx, &b, id, projection)
			values = append(values, super.NewValue(typ, b.Bytes().Body()))
		}
	} else {
		typ := metadataValue(o.cctx, sctx, &b, o.Root(), projection)
		values = append(values, super.NewValue(typ, b.Bytes().Body()))
	}
	return values
}
