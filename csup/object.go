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
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio/bsupio"
)

type Object struct {
	readerAt io.ReaderAt
	header   Header
	meta     Metadata //XXX get rid of this
	metaval  *super.Value
}

func NewObject(r io.ReaderAt) (*Object, error) {
	hdr, err := ReadHeader(io.NewSectionReader(r, 0, HeaderSize))
	if err != nil {
		return nil, err
	}
	val, err := readMetadataRaw(super.NewContext(), io.NewSectionReader(r, HeaderSize, int64(hdr.MetaSize)))
	if err != nil {
		return nil, err
	}
	return &Object{
		readerAt: io.NewSectionReader(r, int64(HeaderSize+hdr.MetaSize), int64(hdr.DataSize)),
		header:   hdr,
		metaval:  val,
	}, nil
}

func (o *Object) Close() error {
	if closer, ok := o.readerAt.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (o *Object) Metadata() Metadata {
	if o.meta == nil {
		if err := o.unmarshal(); err != nil {
			panic(err) //XXX
		}
	}
	fmt.Println("REMOVE csup.Object.Metadata")
	return o.meta
}

func (o *Object) MetadataAsValue() *super.Value {
	return o.metaval
}

func (o *Object) DataReader() io.ReaderAt {
	return o.readerAt
}

func (o *Object) Size() uint64 {
	return HeaderSize + o.header.MetaSize + o.header.DataSize
}

func (o *Object) unmarshal() error {
	sctx := super.NewContext()
	u := sup.NewBSUPUnmarshaler()
	u.SetContext(sctx)
	u.Bind(Template...)
	if err := u.Unmarshal(*o.metaval, &o.meta); err != nil {
		return err
	}
	return nil
}

func readMetadataRaw(sctx *super.Context, r io.Reader) (*super.Value, error) {
	zr := bsupio.NewReader(sctx, r)
	defer zr.Close()
	val, err := zr.Read()
	if err != nil {
		return nil, err
	}
	// Read another val to make sure there is no extra stuff after the metadata.
	if extra, _ := zr.Read(); extra != nil {
		return nil, errors.New("corrupt CSUP: metadata section has more than one Zed value")
	}
	return val, nil
}
