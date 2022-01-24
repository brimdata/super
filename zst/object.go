// Package zst implements reading and writing zst storage objects
// to and from zng row format.  The zst storage format consists of
// a section of column data stored in zng values followed by a section
// containing a zng record stream comprised of N zng "reassembly records"
// (one for each zed.TypeRecord or "schema") stored in the zst object, plus
// an N+1st zng record describing the list of schemas IDs of the original
// zng rows that were encoded into the zst object.
//
// A zst storage object must be seekable (e.g., a local file or s3 object),
// so, unlike zng, streaming of zst objects is not supported.
//
// The zst/column package handles reading and writing row data to columns,
// while the zst package comprises the API used to read and write zst objects.
//
// An Object provides the interface to the underlying storage object.
// To generate rows or cuts (and in the future more sophisticated traversals
// and introspection), an Assembly is created from the Object then zng records
// are read from the assembly, which implements zio.Reader.  The Assembly
// keeps track of where each column is, which is why you need a separate
// Assembly per scan.
//
// You can have multiple Assembly's referring to one Object as once an
// object is created, it's state never changes.  That said, each assembly
// will issue reads to the underlying storage object and the read pattern
// may create performance issues.
package zst

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
	"github.com/brimdata/zed/zst/column"
)

type Object struct {
	seeker   *storage.Seeker
	closer   io.Closer
	zctx     *zed.Context
	assembly *Assembly
	meta     FileMeta
	sections []int64
	size     int64
	builder  zcode.Builder
	err      error
}

func NewObject(zctx *zed.Context, s *storage.Seeker, size int64) (*Object, error) {
	meta, sections, err := readTrailer(s, size)
	if err != nil {
		return nil, err
	}
	if meta.SkewThresh > MaxSkewThresh {
		return nil, fmt.Errorf("skew threshold too large (%d)", meta.SkewThresh)
	}
	if meta.SegmentThresh > MaxSegmentThresh {
		return nil, fmt.Errorf("column threshold too large (%d)", meta.SegmentThresh)
	}
	o := &Object{
		seeker:   s,
		zctx:     zctx,
		meta:     *meta,
		sections: sections,
		size:     size,
	}
	o.assembly, err = o.readAssembly()
	return o, err
}

func NewObjectFromSeeker(zctx *zed.Context, s *storage.Seeker) (*Object, error) {
	size, err := storage.Size(s.Reader)
	if err != nil {
		return nil, err
	}
	return NewObject(zctx, s, size)
}

func NewObjectFromPath(ctx context.Context, zctx *zed.Context, engine storage.Engine, path string) (*Object, error) {
	uri, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	r, err := engine.Get(ctx, uri)
	if err != nil {
		return nil, err
	}
	size, err := storage.Size(r)
	if err != nil {
		return nil, err
	}
	seeker, err := storage.NewSeeker(r)
	if err != nil {
		return nil, err
	}
	o, err := NewObject(zctx, seeker, size)
	if err == nil {
		o.closer = r
	}
	return o, err
}

func (o *Object) Close() error {
	if o.closer != nil {
		return o.closer.Close()
	}
	return nil
}

func (o *Object) IsEmpty() bool {
	return o.sections == nil
}

func (o *Object) readAssembly() (*Assembly, error) {
	reader := o.NewReassemblyReader()
	assembly := &Assembly{}
	var val *zed.Value
	for {
		var err error
		val, err = reader.Read()
		if err != nil {
			return nil, err
		}
		if val == nil {
			return nil, errors.New("zst: corrupt trailer: root ressembly map not found")
		}
		if !val.IsNull() {
			break
		}
		assembly.types = append(assembly.types, val.Type)
	}
	assembly.root = *val.Copy()
	expectedType, err := zson.ParseType(o.zctx, column.SegmapTypeString)
	if err != nil {
		return nil, err
	}
	if assembly.root.Type != expectedType {
		return nil, fmt.Errorf("zst root reassembly value has wrong type: %s; should be %s", assembly.root.Type, expectedType)
	}
	for range assembly.types {
		val, err := reader.Read()
		if err != nil {
			return nil, err
		}
		if val == nil {
			return nil, errors.New("zst: corrupt reassembly section: number of reassembly maps not equal to number of types")
		}
		assembly.maps = append(assembly.maps, val.Copy())
	}
	if val, _ = reader.Read(); val != nil {
		return nil, errors.New("zst: corrupt reassembly section: numer of reassembly maps exceeds number of types")
	}
	return assembly, nil
}

func (o *Object) section(level int) (int64, int64) {
	off := int64(0)
	for k := 0; k < level; k++ {
		off += o.sections[k]
	}
	return off, o.sections[level]
}

func (o *Object) newSectionReader(level int, sectionOff int64) zio.Reader {
	off, len := o.section(level)
	off += sectionOff
	len -= sectionOff
	reader := io.NewSectionReader(o.seeker, off, len)
	return zngio.NewReader(reader, o.zctx)
}

func (o *Object) NewReassemblyReader() zio.Reader {
	return o.newSectionReader(1, 0)
}
