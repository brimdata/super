package data

import (
	"context"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/lake/seekindex"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/bufwriter"
	"github.com/brimdata/zed/pkg/field"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio/zngio"
)

// Writer is a zio.Writer that writes a stream of sorted records into a
// data object.
type Writer struct {
	object           *Object
	byteCounter      *writeCounter
	count            uint64
	writer           *zngio.Writer
	order            order.Which
	seekIndex        *seekindex.Writer
	seekIndexStride  int
	seekIndexTrigger int
	first            bool
	seekMin          *zed.Value
	poolKey          field.Path
	keyArena         *zed.Arena
	seekMinArena     *zed.Arena
	maxArena         *zed.Arena
}

// NewWriter returns a writer for writing the data of a zng-row storage object as
// well as optionally creating a seek index for the row object when the
// seekIndexStride is non-zero.  We assume all records are non-volatile until
// Close as zed.Values from the various record bodies are referenced across
// calls to Write.
func (o *Object) NewWriter(ctx context.Context, engine storage.Engine, path *storage.URI, order order.Which, poolKey field.Path, seekIndexStride int) (*Writer, error) {
	out, err := engine.Put(ctx, o.SequenceURI(path))
	if err != nil {
		return nil, err
	}
	counter := &writeCounter{bufwriter.New(out), 0}
	w := &Writer{
		object:       o,
		byteCounter:  counter,
		writer:       zngio.NewWriter(counter),
		order:        order,
		poolKey:      poolKey,
		first:        true,
		keyArena:     zed.NewArena(),
		seekMinArena: zed.NewArena(),
		maxArena:     zed.NewArena(),
	}
	if seekIndexStride == 0 {
		seekIndexStride = DefaultSeekStride
	}
	w.seekIndexStride = seekIndexStride
	seekOut, err := engine.Put(ctx, o.SeekIndexURI(path))
	if err != nil {
		return nil, err
	}
	w.seekIndex = seekindex.NewWriter(zngio.NewWriter(bufwriter.New(seekOut)))
	return w, nil
}

func (w *Writer) Write(val zed.Value) error {
	w.keyArena.Reset()
	key := val.DerefPath(w.keyArena, w.poolKey).MissingAsNull()
	return w.WriteWithKey(key, val)
}

func (w *Writer) WriteWithKey(key, val zed.Value) error {
	w.count++
	if err := w.writer.Write(val); err != nil {
		return err
	}
	w.maxArena.Reset()
	w.object.Max = key.Copy(w.maxArena)
	return w.writeIndex(key)
}

func (w *Writer) writeIndex(key zed.Value) error {
	w.seekIndexTrigger += len(key.Bytes())
	if w.first {
		w.first = false
		w.object.Arena = zed.NewArena()
		w.object.Min = key.Copy(w.object.Arena)
	}
	if w.seekMin == nil {
		w.seekMinArena, w.keyArena = w.keyArena, w.seekMinArena
		w.seekMin = &key
	}
	if w.seekIndexTrigger < w.seekIndexStride {
		return nil
	}
	if err := w.writer.EndStream(); err != nil {
		return err
	}
	return w.flushSeekIndex()
}

func (w *Writer) flushSeekIndex() error {
	if w.seekMin != nil {
		w.seekIndexTrigger = 0
		min := *w.seekMin
		max := w.object.Max
		if w.order == order.Desc {
			min, max = max, min
		}
		w.seekMin = nil
		return w.seekIndex.Write(min, max, w.count, uint64(w.writer.Position()))
	}
	return nil
}

// Abort is called when an error occurs during write. Errors are ignored
// because the write error will be more informative and should be returned.
func (w *Writer) Abort() {
	w.writer.Close()
	w.seekIndex.Close()
}

func (w *Writer) Close(ctx context.Context) error {
	if err := w.writer.Close(); err != nil {
		w.Abort()
		return err
	}
	if err := w.flushSeekIndex(); err != nil {
		w.Abort()
		return err
	}
	if err := w.seekIndex.Close(); err != nil {
		w.Abort()
		return err
	}
	w.object.Count = w.count
	w.object.Size = w.writer.Position()
	w.object.Max = w.object.Max.Copy(w.object.Arena)
	if w.order == order.Desc {
		w.object.Min, w.object.Max = w.object.Max, w.object.Min
	}
	return nil
}

func (w *Writer) BytesWritten() int64 {
	return w.byteCounter.size
}

func (w *Writer) RecordsWritten() uint64 {
	return w.count
}

// Object returns the Object written by the writer. This is only valid after
// Close() has returned a nil error.
func (w *Writer) Object() *Object {
	return w.object
}

type writeCounter struct {
	io.WriteCloser
	size int64
}

func (w *writeCounter) Write(b []byte) (int, error) {
	n, err := w.WriteCloser.Write(b)
	w.size += int64(n)
	return n, err
}
