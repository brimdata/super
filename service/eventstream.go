package service

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
)

type event struct {
	name  string
	value super.Value
}

type eventStreamWriter struct {
	body   io.Writer
	format string
}

func (e *eventStreamWriter) writeEvent(ev event) error {
	var buf bytes.Buffer
	w, err := anyio.NewWriter(zio.NopCloser(&buf), anyio.WriterOpts{Format: e.format})
	if err != nil {
		return err
	}
	if err := w.Write(ev.value); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.body, "event: %s\ndata: %s\n\n", ev.name, &buf)
	return err
}
