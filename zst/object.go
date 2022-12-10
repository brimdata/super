// Package zst implements the reading and writing of ZST storage objects
// to and from any Zed format.  The ZST storage format is described
// at https://github.com/brimdata/zed/blob/main/docs/formats/zst.md.
//
// A ZST storage object must be seekable (e.g., a local file or S3 object),
// so, unlike ZNG, streaming of ZST objects is not supported.
//
// The zst/vector package handles reading and writing Zed sequence data to vectors,
// while the zst package comprises the API used to read and write ZST objects.
package zst

import (
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
	"github.com/brimdata/zed/zst/vector"
)

type Object struct {
	readerAt io.ReaderAt
	zctx     *zed.Context
	root     []vector.Segment
	maps     []vector.Metadata
	trailer  FileMeta
	sections []int64
	size     int64
}

func NewObject(zctx *zed.Context, r io.ReaderAt, size int64) (*Object, error) {
	trailer, sections, err := readTrailer(r, size)
	if err != nil {
		return nil, err
	}
	if trailer.SkewThresh > MaxSkewThresh {
		return nil, fmt.Errorf("skew threshold too large (%d)", trailer.SkewThresh)
	}
	if trailer.SegmentThresh > MaxSegmentThresh {
		return nil, fmt.Errorf("vector threshold too large (%d)", trailer.SegmentThresh)
	}
	o := &Object{
		readerAt: r,
		zctx:     zctx,
		trailer:  *trailer,
		sections: sections,
		size:     size,
	}
	if err := o.readMetaData(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *Object) IsEmpty() bool {
	return o.sections == nil
}

func (o *Object) FetchMetadata() ([]int32, []vector.Metadata, error) {
	typeIDs, err := ReadIntVector(o.root, o.readerAt)
	if err != nil {
		return nil, nil, err
	}
	return typeIDs, o.maps, nil
}

func ReadIntVector(segments []vector.Segment, r io.ReaderAt) ([]int32, error) {
	reader := vector.NewInt64Reader(segments, r)
	var out []int32
	for {
		val, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}
		out = append(out, int32(val))
	}
}

func (o *Object) readMetaData() error {
	reader := o.NewReassemblyReader()
	defer reader.Close()
	// First value is the segmap for the root list of type numbers.
	// The type number is relative to the array of maps.
	val, err := reader.Read()
	if err != nil {
		return err
	}
	u := zson.NewZNGUnmarshaler()
	u.SetContext(o.zctx)
	u.Bind(vector.Template...)
	if err := u.Unmarshal(val, &o.root); err != nil {
		return err
	}
	// The rest of the values are vector.Metadata, one for each
	// Zed type that has been encoded into the ZST file.
	for {
		val, err = reader.Read()
		if err != nil {
			return err
		}
		if val == nil {
			break
		}
		var meta vector.Metadata
		if err := u.Unmarshal(val, &meta); err != nil {
			return err
		}
		o.maps = append(o.maps, meta)
	}
	return nil
}

func (o *Object) section(level int) (int64, int64) {
	off := int64(0)
	for k := 0; k < level; k++ {
		off += o.sections[k]
	}
	return off, o.sections[level]
}

func (o *Object) newSectionReader(level int, sectionOff int64) *zngio.Reader {
	off, len := o.section(level)
	off += sectionOff
	len -= sectionOff
	reader := io.NewSectionReader(o.readerAt, off, len)
	return zngio.NewReader(o.zctx, reader)
}

func (o *Object) NewReassemblyReader() *zngio.Reader {
	return o.newSectionReader(1, 0)
}
