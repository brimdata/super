package emitter

import (
	"bytes"

	"github.com/superdb/super/sio"
	"github.com/superdb/super/sio/anyio"
	"github.com/superdb/super/vector/vio"
)

type Bytes struct {
	vio.Pusher
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
	b.Pusher = w
	return b, nil
}
