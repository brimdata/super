package emitter

import (
	"context"
	"io"
	"os"

	"github.com/brimdata/super/pkg/bufwriter"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/pkg/terminal"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/anyio"
)

func NewFileFromPath(ctx context.Context, engine storage.Engine, path string, unbuffered bool, opts anyio.WriterOpts) (sio.WriteCloser, error) {
	if path == "" {
		path = "stdio:stdout"
	}
	uri, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	return NewFileFromURI(ctx, engine, uri, unbuffered, opts)
}

func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && terminal.IsTerminalFile(f)
}

func NewFileFromURI(ctx context.Context, engine storage.Engine, path *storage.URI, unbuffered bool, opts anyio.WriterOpts) (sio.WriteCloser, error) {
	f, err := engine.Put(ctx, path)
	if err != nil {
		return nil, err
	}
	wc := f
	if path.Scheme == "stdio" {
		// Don't close stdio in case we live inside something
		// that has multiple stdio users.
		wc = sio.NopCloser(f)
	}
	if !unbuffered && !IsTerminal(f) {
		// Don't buffer terminal output.
		wc = bufwriter.New(wc)
	}
	// On close, sio.WriteCloser.Close will close and flush the
	// downstream writer, which will flush the bufwriter here and,
	// in turn, close its underlying writer.
	return anyio.NewWriter(wc, opts)
}
