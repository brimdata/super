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
// Builder and passing in a zcode.Builder to reconstruct the super value.
package csup

import (
	"errors"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio/zngio"
	"github.com/brimdata/super/zson"
)

type Object struct {
	readerAt io.ReaderAt
	header   Header
	meta     Metadata
}

func NewObject(r io.ReaderAt) (*Object, error) {
	hdr, err := ReadHeader(io.NewSectionReader(r, 0, HeaderSize))
	if err != nil {
		return nil, err
	}
	meta, err := readMetadata(io.NewSectionReader(r, HeaderSize, int64(hdr.MetaSize)))
	if err != nil {
		return nil, err
	}
	return &Object{
		readerAt: io.NewSectionReader(r, int64(HeaderSize+hdr.MetaSize), int64(hdr.DataSize)),
		header:   hdr,
		meta:     meta,
	}, nil
}

func (o *Object) Close() error {
	if closer, ok := o.readerAt.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (o *Object) Metadata() Metadata {
	return o.meta
}

func (o *Object) DataReader() io.ReaderAt {
	return o.readerAt
}

func (o *Object) Size() uint64 {
	return HeaderSize + o.header.MetaSize + o.header.DataSize
}

func readMetadata(r io.Reader) (Metadata, error) {
	zctx := super.NewContext()
	zr := zngio.NewReader(zctx, r)
	defer zr.Close()
	val, err := zr.Read()
	if err != nil {
		return nil, err
	}
	u := zson.NewZNGUnmarshaler()
	u.SetContext(zctx)
	u.Bind(Template...)
	var meta Metadata
	if err := u.Unmarshal(*val, &meta); err != nil {
		return nil, err
	}
	// Read another val to make sure there is no extra stuff after the metadata.
	if extra, _ := zr.Read(); extra != nil {
		return nil, errors.New("corrupt CSUP: metadata section has more than one Zed value")
	}
	return meta, nil
}
