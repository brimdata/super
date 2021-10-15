package zst

import (
	"context"
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/bufwriter"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zst/column"
)

const (
	MaxSegmentThresh = column.MaxSegmentThresh
	MaxSkewThresh    = 512 * 1024 * 1024
)

// Writer implements the zio.Writer interface. A Writer creates a columnar
// zst object from a stream of zed.Records.
type Writer struct {
	zctx       *zed.Context
	writer     io.WriteCloser
	spiller    *column.Spiller
	schemaMap  map[int]int
	schemas    []column.RecordWriter
	types      []zed.Type
	skewThresh int
	segThresh  int
	// We keep track of the size of rows we've encoded into in-memory
	// data structures.  This is roughtly propertional to the amount of
	// memory used and the max amount of skew between rows that will be
	// needed for reader-side buffering.  So when the memory footprint
	// exceeds the confired skew theshhold, we flush the columns to storage.
	footprint int
	root      *column.IntWriter
}

func NewWriter(w io.WriteCloser, skewThresh, segThresh int) (*Writer, error) {
	if err := checkThresh("skew", MaxSkewThresh, skewThresh); err != nil {
		return nil, err
	}
	if err := checkThresh("column", MaxSegmentThresh, segThresh); err != nil {
		return nil, err
	}
	spiller := column.NewSpiller(w, segThresh)
	return &Writer{
		zctx:       zed.NewContext(),
		spiller:    spiller,
		writer:     w,
		schemaMap:  make(map[int]int),
		skewThresh: skewThresh,
		segThresh:  segThresh,
		root:       column.NewIntWriter(spiller),
	}, nil
}

func (w *Writer) Close() error {
	firstErr := w.finalize()
	if err := w.writer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

type WriterURI struct {
	Writer
	engine storage.Engine
	uri    *storage.URI
}

func NewWriterFromPath(ctx context.Context, engine storage.Engine, path string, skewThresh, segThresh int) (*WriterURI, error) {
	uri, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	w, err := engine.Put(ctx, uri)
	if err != nil {
		return nil, err
	}
	writer, err := NewWriter(bufwriter.New(w), skewThresh, segThresh)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", uri, err)
	}
	return &WriterURI{
		Writer: *writer,
		engine: engine,
		uri:    uri,
	}, nil
}

func checkThresh(which string, max, thresh int) error {
	if thresh == 0 {
		return fmt.Errorf("zst %s threshold cannot be zero", which)
	}
	if thresh > max {
		return fmt.Errorf("zst %s threshold too large (%d)", which, thresh)
	}
	return nil
}

func (w *Writer) Write(rec *zed.Value) error {
	inputID := zed.TypeID(rec.Type)
	schemaID, ok := w.schemaMap[inputID]
	if !ok {
		schema, err := w.zctx.TranslateType(rec.Type)
		if err != nil {
			return err
		}
		recType := zed.TypeRecordOf(schema)
		schemaID = len(w.schemas)
		w.schemaMap[inputID] = schemaID
		rw := column.NewRecordWriter(recType, w.spiller)
		w.schemas = append(w.schemas, rw)
		w.types = append(w.types, schema)
	}
	if err := w.root.Write(int32(schemaID)); err != nil {
		return err
	}
	if err := w.schemas[schemaID].Write(rec.Bytes); err != nil {
		return err
	}
	w.footprint += len(rec.Bytes)
	if w.footprint >= w.skewThresh {
		w.footprint = 0
		return w.flush(false)
	}
	return nil
}

// Abort closes this writer, deleting any and all objects and/or files associated
// with it.
func (w *WriterURI) Abort(ctx context.Context) error {
	firstErr := w.writer.Close()
	if err := w.engine.DeleteByPrefix(ctx, w.uri); firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (w *WriterURI) Close() error {
	if err := w.finalize(); err != nil {
		w.writer.Close()
		return err
	}
	return w.writer.Close()
}

func (w *Writer) flush(eof bool) error {
	for _, col := range w.schemas {
		if err := col.Flush(eof); err != nil {
			return err
		}
	}
	return w.root.Flush(eof)
}

func (w *Writer) finalize() error {
	if err := w.flush(true); err != nil {
		return err
	}
	// At this point all the column data has been written out
	// to the underlying spiller, so we start writing zng at this point.
	zw := zngio.NewWriter(w.writer, zngio.WriterOpts{})
	dataSize := w.spiller.Position()
	var b zcode.Builder
	// First, write out empty records for each schemas.  Since these types
	// are all put here first, in the orde they were originally encountered
	// in the zng input, when they are read fresh by the zst reader, the
	// reconstructued type context will exactly match the original context
	// and the resulting zng output will be byte-equivalent to the original
	// input.
	for _, schema := range w.types {
		b.Reset()
		for _, col := range zed.TypeRecordOf(schema).Columns {
			if zed.IsContainerType(col.Type) {
				b.AppendContainer(nil)
			} else {
				b.AppendPrimitive(nil)
			}
		}
		rec := zed.NewRecord(schema, b.Bytes())
		if err := zw.Write(rec); err != nil {
			return err
		}
	}
	// Next, write the root reassembly record.
	b.Reset()
	typ, err := w.root.MarshalZNG(w.zctx, &b)
	if err != nil {
		return err
	}
	rootType, err := w.zctx.LookupTypeRecord([]zed.Column{{"root", typ}})
	if err != nil {
		return err
	}
	rec := zed.NewRecord(rootType, b.Bytes())
	if err := zw.Write(rec); err != nil {
		return err
	}
	// Now, write out the reassembly record for each schema.  Each record
	// is highly nested and encodes all of the segmaps for every column stream
	// needed to reconstruct all of the records of that schema.
	for _, schema := range w.schemas {
		b.Reset()
		typ, err := schema.MarshalZNG(w.zctx, &b)
		if err != nil {
			return err
		}
		body, err := b.Bytes().ContainerBody()
		if err != nil {
			return err
		}
		rec := zed.NewRecord(typ.(*zed.TypeRecord), body)
		if err := zw.Write(rec); err != nil {
			return err
		}
	}
	zw.EndStream()
	columnSize := zw.Position()
	sizes := []int64{dataSize, columnSize}
	return writeTrailer(zw, w.zctx, w.skewThresh, w.segThresh, sizes)
}

func (w *Writer) writeEmptyTrailer() error {
	zw := zngio.NewWriter(w.writer, zngio.WriterOpts{})
	return writeTrailer(zw, w.zctx, w.skewThresh, w.segThresh, nil)
}

func writeTrailer(w *zngio.Writer, zctx *zed.Context, skewThresh, segThresh int, sizes []int64) error {
	rec, err := newTrailerRecord(zctx, skewThresh, segThresh, sizes)
	if err != nil {
		return err
	}
	if err := w.Write(rec); err != nil {
		return err
	}
	return w.EndStream()
}
