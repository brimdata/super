package exec

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

// Query runs a flowgraph as a sbuf.Puller and implements a Close() method
// that gracefully tears down the flowgraph.  Its AsReader() and AsProgressReader()
// methods provide a convenient means to run a flowgraph as sio.Reader.
type Query struct {
	vio.Puller
	rctx  *runtime.Context
	meter vio.Meter
}

var _ runtime.Query = (*Query)(nil)

func NewQuery(rctx *runtime.Context, puller vio.Puller, meter vio.Meter) *Query {
	return &Query{
		Puller: puller,
		rctx:   rctx,
		meter:  meter,
	}
}

func (q *Query) AsReader() sio.Reader {
	return sbuf.PullerReader(sbuf.NewMaterializer(q.Puller))
}

func (q *Query) AsPuller() sbuf.Puller {
	return sbuf.NewMaterializer(q.Puller)
}

func (q *Query) Progress() vio.Progress {
	return q.meter.Progress()
}

func (q *Query) Meter() vio.Meter {
	return q.meter
}

func (q *Query) Close() error {
	q.rctx.Cancel()
	return nil
}

func (q *Query) Pull(done bool) (vector.Any, error) {
	if done {
		q.rctx.Cancel()
	}
	return q.Puller.Pull(done)
}
