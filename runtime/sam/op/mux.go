package op

import (
	"context"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op/debug"
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
	rctx      *runtime.Context
	once      sync.Once
	ch        <-chan result
	debugCh   <-chan result
	parents   []*puller
	nparents  int
	debuggers []*debugger
	ndebugs   int
	doneCh    chan struct{}
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

type debugger struct {
	valCh    <-chan super.Value
	doneCh   <-chan struct{}
	resultCh chan<- result
	label    string
}

func (d *debugger) run() {
	for {
		select {
		case val := <-d.valCh:
			batch := sbuf.NewArray([]super.Value{val})
			d.resultCh <- result{batch, d.label, nil}
		case <-d.doneCh:
			d.resultCh <- result{}
			return
		}
	}
}

func NewMux(rctx *runtime.Context, parents map[string]sbuf.Puller, debugs []*debug.Op) *Mux {
	if len(parents)+len(debugs) <= 1 {
		panic("mux.New() must be called with two or more parents")
	}
	ch := make(chan result)
	debugCh := make(chan result)
	doneCh := make(chan struct{})
	pullers := make([]*puller, 0, len(parents))
	for label, parent := range parents {
		pullers = append(pullers, &puller{NewCatcher(parent), ch, label})
	}
	debuggers := make([]*debugger, 0, len(debugs))
	for _, d := range debugs {
		// XXX for now, put the debug output on main.  there should be a way
		// to wire these up differently, e.g., to send debug to stderr.
		// perhaps by default on terminal, we send "debug" channel to stderr.
		debuggers = append(debuggers, &debugger{d.Channel(), doneCh, debugCh, "debug"})
	}
	return &Mux{
		rctx:      rctx,
		ch:        ch,
		debugCh:   debugCh,
		doneCh:    doneCh,
		parents:   pullers,
		nparents:  len(parents),
		debuggers: debuggers,
		ndebugs:   len(debuggers),
	}
}

// Pull implements the merge logic for returning data from the upstreams.
func (m *Mux) Pull(bool) (sbuf.Batch, error) {
	if m.nparents == 0 {
		if m.ndebugs != 0 {
			select {
			case res := <-m.debugCh:
				batch := res.batch
				err := res.err
				if err != nil {
					m.rctx.Cancel()
					return nil, err
				}
				if batch != nil {
					batch = sbuf.Label(res.label, batch)
				} else {
					eoc := sbuf.EndOfChannel(res.label)
					batch = &eoc
					m.ndebugs--
				}
				return batch, err
			case <-m.rctx.Context.Done():
				return nil, m.rctx.Context.Err()
			}
		}
		// When we get to EOS and all debugs are done, we make sure all
		// the flowgraph goroutines terminate by canceling the proc context.
		m.rctx.Cancel()
		return nil, nil
	}
	m.once.Do(func() {
		for _, puller := range m.parents {
			go puller.run(m.rctx.Context)
		}
		for _, debugger := range m.debuggers {
			go debugger.run()
		}
	})
	for {
		select {
		case res := <-m.ch:
			batch := res.batch
			err := res.err
			if err != nil {
				m.rctx.Cancel()
				return nil, err
			}
			if batch != nil {
				batch = sbuf.Label(res.label, batch)
			} else {
				eoc := sbuf.EndOfChannel(res.label)
				batch = &eoc
				m.nparents--
				if m.nparents == 0 {
					close(m.doneCh)
				}
			}
			return batch, err
		case res := <-m.debugCh:
			batch := res.batch
			err := res.err
			if err != nil {
				m.rctx.Cancel()
				return nil, err
			}
			if batch != nil {
				batch = sbuf.Label(res.label, batch)
			} else {
				eoc := sbuf.EndOfChannel(res.label)
				batch = &eoc
				m.ndebugs--
			}
			return batch, err
		case <-m.rctx.Context.Done():
			return nil, m.rctx.Context.Err()
		}
	}
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
