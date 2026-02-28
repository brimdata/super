package op

import (
	"context"
	"sync"

	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sbuf"
)

// Mux implements the muxing of a set of parallel paths at the output of
// a flowgraph.  It also implements the double-EOS algorithm with proc.Latch
// to detect the end of each parallel stream.  Its output protocol is a single EOS
// when all of the upstream legs are done at which time it cancels the flowgraoh.
// Each  batch returned by the mux is wrapped in a Batch, which can be unwrappd
// with Unwrap to extract the integer index of the output (in left-to-right
// DFS traversal order of the flowgraph).  This proc requires more than one
// parent; use proc.Latcher for a single-output flowgraph.
type Mux struct {
	rctx     *runtime.Context
	once     sync.Once
	ch       <-chan result
	parents  []*puller
	nparents int
	debugger *debugger
}

type result struct {
	batch sbuf.Batch
	label string
	err   error
}

type puller struct {
	sbuf.Puller
	ch    chan<- result
	label string
}

func (p *puller) run(ctx context.Context) {
	for {
		batch, err := p.Pull(false)
		select {
		case p.ch <- result{batch, p.label, err}:
			if batch == nil || err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func NewMux(rctx *runtime.Context, parents map[string]sbuf.Puller, debugs []<-chan sbuf.Batch) *Mux {
	if len(parents)+len(debugs) <= 1 {
		panic("mux.New() must be called with two or more parents")
	}
	ch := make(chan result)
	pullers := make([]*puller, 0, len(parents))
	for label, parent := range parents {
		pullers = append(pullers, &puller{NewCatcher(parent), ch, label})
	}
	var debugger *debugger
	if len(debugs) != 0 {
		debugger = newDebugger(debugs)
	}
	return &Mux{
		rctx:     rctx,
		ch:       ch,
		parents:  pullers,
		nparents: len(parents),
		debugger: debugger,
	}
}

// Pull implements the merge logic for returning data from the upstreams.
func (m *Mux) Pull(bool) (sbuf.Batch, error) {
	if m.nparents == 0 {
		if m.debugger.active() {
			res := m.debugger.pull(m.rctx.Context)
			batch, _, err := labelBatch(res)
			if err != nil {
				m.rctx.Cancel()
			}
			return batch, err
		}
		// When we get to EOS and all debugs are done, we make sure all
		// the flowgraph goroutines terminate by canceling the global context.
		m.rctx.Cancel()
		return nil, nil
	}
	m.once.Do(func() {
		for _, puller := range m.parents {
			go puller.run(m.rctx.Context)
		}
		if m.debugger != nil {
			m.debugger.run()
		}
	})
	for {
		select {
		case res := <-m.ch:
			batch, eoc, err := labelBatch(res)
			if err != nil {
				m.rctx.Cancel()
				return nil, err
			}
			if eoc {
				m.nparents--
				if m.nparents == 0 {
					m.debugger.shutdown()
				}
			}
			return batch, err
		case res := <-m.debugger.channel():
			m.debugger.check(res)
			batch, _, err := labelBatch(res)
			if err != nil {
				m.rctx.Cancel()
				return nil, err
			}
			return batch, err
		case <-m.rctx.Context.Done():
			return nil, m.rctx.Context.Err()
		}
	}
}

func labelBatch(r result) (sbuf.Batch, bool, error) {
	if r.err != nil {
		return nil, false, r.err
	}
	var end bool
	batch := r.batch
	if batch != nil {
		batch = sbuf.Label(r.label, batch)
	} else {
		eoc := sbuf.EndOfChannel(r.label)
		batch = &eoc
		end = true
	}
	return batch, end, nil
}

type Single struct {
	sbuf.Puller
	label string
	eos   bool
}

func NewSingle(label string, parent sbuf.Puller) *Single {
	return &Single{Puller: parent, label: label}
}

func (s *Single) Pull(bool) (sbuf.Batch, error) {
	if s.eos {
		return nil, nil
	}
	batch, err := s.Puller.Pull(false)
	if batch == nil {
		s.eos = true
		eoc := sbuf.EndOfChannel(s.label)
		return &eoc, err
	}
	return sbuf.Label(s.label, batch), err
}
