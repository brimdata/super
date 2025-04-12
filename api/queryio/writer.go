package queryio

import (
	"io"
	"net/http"

	"github.com/brimdata/super/api"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
	"github.com/brimdata/super/zio/jsonio"
)

type controlWriter interface {
	zio.WriteCloser
	WriteControl(any) error
}

type Writer struct {
	channel string
	start   nano.Ts
	writer  zio.WriteCloser
	ctrl    bool
	flusher http.Flusher
}

func NewWriter(w io.WriteCloser, format string, flusher http.Flusher, ctrl bool) (*Writer, error) {
	d := &Writer{
		ctrl:    ctrl,
		start:   nano.Now(),
		flusher: flusher,
	}
	var err error
	switch format {
	case "bsup":
		d.writer = NewBSUPWriter(w)
	case "zjson":
		d.writer = NewZJSONWriter(w)
	case "json":
		// A JSON response is always an array.
		d.writer = jsonio.NewArrayWriter(w)
	case "ndjson":
		d.writer = jsonio.NewWriter(w, jsonio.WriterOpts{})
	default:
		d.writer, err = anyio.NewWriter(zio.NopCloser(w), anyio.WriterOpts{
			Format: format,
		})
	}
	return d, err
}

func (w *Writer) WriteBatch(channel string, batch zbuf.Batch) error {
	if w.channel != channel {
		w.channel = channel
		if err := w.WriteControl(api.QueryChannelSet{Channel: channel}); err != nil {
			return err
		}
	}
	defer batch.Unref()
	return zbuf.WriteBatch(w.writer, batch)
}

func (w *Writer) WhiteChannelEnd(channel string) error {
	return w.WriteControl(api.QueryChannelEnd{Channel: channel})
}

func (w *Writer) WriteProgress(stats zbuf.Progress) error {
	v := api.QueryStats{
		StartTime:  w.start,
		UpdateTime: nano.Now(),
		Progress:   stats,
	}
	return w.WriteControl(v)
}

func (w *Writer) WriteError(err error) {
	w.WriteControl(api.QueryError{Error: err.Error()})
}

func (w *Writer) WriteControl(value any) error {
	if !w.ctrl {
		return nil
	}
	var err error
	if ctrl, ok := w.writer.(controlWriter); ok {
		err = ctrl.WriteControl(value)
		if w.flusher != nil {
			w.flusher.Flush()
		}
	}
	return err
}

func (w *Writer) Close() error {
	return w.writer.Close()
}
