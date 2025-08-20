package emitter

import (
	"bytes"

	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/anyio"
)

type Bytes struct {
	sio.Writer
	buf bytes.Buffer
}

func (b *Bytes) Bytes() []byte {
	return b.buf.Bytes()
}

func NewBytes(opts anyio.WriterOpts) (*Bytes, error) {
	b := &Bytes{}
	w, err := anyio.NewWriter(sio.NopCloser(&b.buf), opts)
	if err != nil {
		return nil, err
	}
	b.Writer = w
	return b, nil
}
