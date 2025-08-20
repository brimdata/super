package exec

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/zbuf"
)

// Query runs a flowgraph as a zbuf.Puller and implements a Close() method
// that gracefully tears down the flowgraph.  Its AsReader() and AsProgressReader()
// methods provide a convenient means to run a flowgraph as sio.Reader.
type Query struct {
	zbuf.Puller
	rctx  *runtime.Context
	meter zbuf.Meter
}

var _ runtime.Query = (*Query)(nil)

func NewQuery(rctx *runtime.Context, puller zbuf.Puller, meter zbuf.Meter) *Query {
	return &Query{
		Puller: puller,
		rctx:   rctx,
		meter:  meter,
	}
}

func (q *Query) AsReader() sio.Reader {
	return zbuf.PullerReader(q)
}

func (q *Query) Progress() zbuf.Progress {
	return q.meter.Progress()
}

func (q *Query) Meter() zbuf.Meter {
	return q.meter
}

func (q *Query) Close() error {
	q.rctx.Cancel()
	return nil
}

func (q *Query) Pull(done bool) (zbuf.Batch, error) {
	if done {
		q.rctx.Cancel()
	}
	return q.Puller.Pull(done)
}
