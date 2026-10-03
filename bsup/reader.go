package bsup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/superdb/super"
	"github.com/superdb/super/pkg/field"
	"github.com/superdb/super/scode"
	"github.com/superdb/super/vector"
)

type FrameReader interface {
	frameType()
}

func (*ColumnReader) frameType() {}
func (*RowReader) frameType()    {}

type ColumnReader struct {
	cctx       *Context
	readerAt   io.ReaderAt
	dataReader io.ReaderAt
	header     *ColumnHeader
}

func newColumnReader(r io.ReaderAt, header *ColumnHeader) (*ColumnReader, error) {
	c := &ColumnReader{
		cctx:     NewContext(),
		readerAt: r,
		header:   header,
	}
	off, size := header.MetadataSection()
	if err := c.cctx.readMeta(io.NewSectionReader(r, off, size)); err != nil {
		return nil, err
	}
	if header.Root >= uint32(len(c.cctx.values)) {
		return nil, fmt.Errorf("root ID %d larger than BSUP context table (len %d)", header.Root, len(c.cctx.values))
	}
	off, size = header.TypedefsSection()
	c.cctx.subtypesReader = io.NewSectionReader(r, off, size)
	c.cctx.subtypesSize = size
	off, size = header.SegmentsSection()
	c.dataReader = io.NewSectionReader(r, off, size)
	return c, nil
}

func (c *ColumnReader) Close() error {
	if closer, ok := c.readerAt.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (c *ColumnReader) Context() *Context {
	return c.cctx
}

func (c *ColumnReader) Root() ID {
	return ID(c.header.Root)
}

func (c *ColumnReader) DataReader() io.ReaderAt {
	return c.dataReader
}

func (c *ColumnReader) IsControl() bool {
	return c.header.OOB
}

func (c *ColumnReader) ProjectMetadata(sctx *super.Context, projection field.Projection) []super.Value {
	var b scode.Builder
	var values []super.Value
	root := c.cctx.Lookup(c.Root())
	if root, ok := root.(*Dynamic); ok {
		for _, id := range root.Values {
			b.Reset()
			typ := metadataValue(c.cctx, sctx, &b, id, projection)
			values = append(values, super.NewValue(typ, b.Bytes().Body()))
		}
	} else {
		typ := metadataValue(c.cctx, sctx, &b, c.Root(), projection)
		values = append(values, super.NewValue(typ, b.Bytes().Body()))
	}
	return values
}

type RowReader struct {
	sctx     *super.Context
	readerAt io.ReaderAt
	header   *RowHeader
}

func newRowReader(sctx *super.Context, r io.ReaderAt, header *RowHeader) (*RowReader, error) {
	return &RowReader{
		sctx:     sctx,
		readerAt: r,
		header:   header,
	}, nil
}

func (r *RowReader) Pull() (vector.Any, error) {
	// We keep things simple and decode the entire frame into one
	// vector that is returned here.  Each typedefs is independent of
	// other frames and we merely need to map the local typedefs
	// to query sctx.
	off, size := r.header.TypedefsSection()
	typedefBytes := make([]byte, size)
	if err := readHeaderBytes(r.readerAt, off, typedefBytes, "row typedefs section"); err != nil {
		return nil, err
	}
	typedefs, ok := super.NewTypeDefsFromBytes(typedefBytes)
	if !ok {
		return nil, errors.New("corrupt row typedefs")
	}
	mapper := super.NewTypeDefsMapper(r.sctx, typedefs)
	off, size = r.header.DataSection()
	buf := make([]byte, size)
	if err := readHeaderBytes(r.readerAt, off, buf, "row typedefs section"); err != nil {
		return nil, err
	}
	builder := vector.NewDynamicValueBuilder()
	for len(buf) > 0 {
		id, n := binary.Uvarint(buf)
		if n <= 0 {
			return nil, errors.New("corrupt row data")
		}
		buf = buf[n:]
		bytesLen, n := binary.Uvarint(buf)
		if n <= 0 {
			return nil, errors.New("corrupt row data")
		}
		buf = buf[n:]
		if bytesLen > uint64(len(buf)) {
			return nil, errors.New("corrupt row data")
		}
		typ := mapper.LookupType(uint32(id))
		if typ == nil {
			return nil, fmt.Errorf("type ID %d not in typedefs table", id)
		}
		builder.Write(super.NewValue(typ, buf[:bytesLen]))
		buf = buf[bytesLen:]
	}
	return builder.Build(r.sctx), nil
}
