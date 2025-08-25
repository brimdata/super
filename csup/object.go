// Package csup implements the reading and writing of CSUP serialization objects.
// The CSUP format is described at https://github.com/brimdata/super/blob/main/docs/formats/csup.md.
//
// A CSUP object is created by allocating an Encoder for any top-level type
// via NewEncoder, which recursively descends into the type, allocating an Encoder
// for each node in the type tree.  The top-level BSUP body is written via a call
// to Write.  Each vector buffers its data in memory until the object is encoded.
//
// After all of the data is written, a metadata section is written consisting
// of a single super value describing the layout of all the vector data obtained by
// calling the Metadata method on the Encoder interface.
//
// Nulls are encoded by a special Nulls object.  Each type is wrapped by a NullsEncoder,
// which run-length encodes alternating sequences of nulls and values.  If no nulls
// are encountered, then the Nulls object is omitted from the metadata.
//
// Data is read from a CSUP object by reading the metadata and creating vector Builders
// for each type by calling NewBuilder with the metadata, which recusirvely creates
// Builders.  An io.ReaderAt is passed to NewBuilder so each vector reader can access
// the underlying storage object and read its vector data effciently in large vector segments.
//
// Once the metadata is assembled in memory, the recontructed sequence data can be
// read from the vector segments by calling the Build method on the top-level
// Builder and passing in a scode.Builder to reconstruct the super value.
package csup

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/scode"
)

type Object struct {
	cctx     *Context
	readerAt io.ReaderAt
	header   Header
}

func NewObject(r io.ReaderAt) (*Object, error) {
	hdr, err := ReadHeader(r)
	if err != nil {
		return nil, err
	}
	return NewObjectFromHeader(r, hdr)
}

func NewObjectFromHeader(r io.ReaderAt, hdr Header) (*Object, error) {
	cctx := NewContext()
	if err := cctx.readMeta(io.NewSectionReader(r, HeaderSize, int64(hdr.MetaSize))); err != nil {
		return nil, err
	}
	if hdr.Root >= uint32(len(cctx.values)) {
		return nil, fmt.Errorf("CSUP root ID %d larger than values table (len %d)", hdr.Root, len(cctx.values))
	}
	return &Object{
		cctx:     cctx,
		readerAt: io.NewSectionReader(r, int64(HeaderSize+hdr.MetaSize), int64(hdr.DataSize)),
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
	return o.header.ObjectSize()
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
