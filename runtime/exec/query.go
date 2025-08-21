package exec

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
)

// Query runs a flowgraph as a sbuf.Puller and implements a Close() method
// that gracefully tears down the flowgraph.  Its AsReader() and AsProgressReader()
// methods provide a convenient means to run a flowgraph as sio.Reader.
type Query struct {
	sbuf.Puller
	rctx  *runtime.Context
	meter sbuf.Meter
}

var _ runtime.Query = (*Query)(nil)

func NewQuery(rctx *runtime.Context, puller sbuf.Puller, meter sbuf.Meter) *Query {
	return &Query{
		Puller: puller,
		rctx:   rctx,
		meter:  meter,
	}
}

func (q *Query) AsReader() sio.Reader {
	return sbuf.PullerReader(q)
}

func (q *Query) Progress() sbuf.Progress {
	return q.meter.Progress()
}

func (q *Query) Meter() sbuf.Meter {
	return q.meter
}

func (q *Query) Close() error {
	q.rctx.Cancel()
	return nil
}

func (q *Query) Pull(done bool) (sbuf.Batch, error) {
	if done {
		q.rctx.Cancel()
	}
	return q.Puller.Pull(done)
}
