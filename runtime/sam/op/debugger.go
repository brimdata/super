package op

import (
	"context"

	"github.com/brimdata/super/sbuf"
)

type debugger struct {
	threads  []dthread
	doneCh   chan struct{}
	resultCh chan result
	label    string
	nrun     int
}

func newDebugger(debugs []<-chan sbuf.Batch) *debugger {
	var threads []dthread
	doneCh := make(chan struct{})
	resultCh := make(chan result)
	for _, ch := range debugs {
		threads = append(threads, dthread{
			batchCh:  ch,
			doneCh:   doneCh,
			resultCh: resultCh,
		})
	}
	return &debugger{
		threads:  threads,
		doneCh:   doneCh,
		resultCh: resultCh,
		nrun:     len(threads),
	}
}

func (d *debugger) active() bool {
	return d != nil && d.nrun != 0
}

func (d *debugger) run() {
	for _, t := range d.threads {
		go t.run()
	}
}

func (d *debugger) shutdown() {
	if d != nil {
		close(d.doneCh)
	}
}

func (d *debugger) pull(ctx context.Context) result {
	select {
	case r := <-d.resultCh:
		d.check(r)
		return r
	case <-ctx.Done():
		return result{err: ctx.Err()}
	}
}

func (d *debugger) check(r result) {
	if r.batch == nil {
		d.nrun--
	}
}

func (d *debugger) channel() <-chan result {
	if d == nil {
		return nil
	}
	return d.resultCh
}

type dthread struct {
	batchCh  <-chan sbuf.Batch
	doneCh   <-chan struct{}
	resultCh chan<- result
}

func (d *dthread) run() {
	for {
		select {
		case batch := <-d.batchCh:
			d.resultCh <- result{batch, "debug", nil}
		case <-d.doneCh:
			d.resultCh <- result{}
			return
		}
	}
}
