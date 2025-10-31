package op

import (
	"fmt"
	"os"
	"sync"

	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
)

type FileScan struct {
	rctx     *runtime.Context
	env      *exec.Environment
	paths    []string
	format   string
	pushdown sbuf.Pushdown

	mu      sync.Mutex
	current exec.VectorConcurrentPuller
	next    int
	numDone int
	puller  vector.Puller
	pullers []*concurrentPuller
}

func NewFileScan(rctx *runtime.Context, env *exec.Environment, paths []string, format string, p sbuf.Pushdown) *FileScan {
	return &FileScan{
		rctx:     rctx,
		env:      env,
		paths:    paths,
		format:   format,
		pushdown: p,
	}
}

func (f *FileScan) Pull(done bool) (vector.Any, error) {
	if f.puller == nil {
		if len(f.pullers) > 0 {
			panic("Pull called after ConcurrentPullers")
		}
		f.puller = f.NewConcurrentPullers(1)[0]
	}
	return f.puller.Pull(done)
}

func (f *FileScan) NewConcurrentPullers(n int) []vector.Puller {
	if n < 1 {
		panic(n)
	}
	if len(f.pullers) > 0 {
		panic("ConcurrentPullers called after Pull or called twice")
	}
	var out []vector.Puller
	for i := range n {
		p := newPuller(f, i)
		f.pullers = append(f.pullers, p)
		out = append(out, p)
	}
	return out
}

func (f *FileScan) done() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.numDone++
	if f.numDone == len(f.pullers) {
		f.current = nil
		f.next = 0
		f.numDone = 0
		for _, p := range f.pullers {
			p.waitCh <- struct{}{}
		}
	}
}

func (f *FileScan) nextFile(current exec.VectorConcurrentPuller) (exec.VectorConcurrentPuller, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if current != f.current {
		return f.current, nil
	}
	for f.next < len(f.paths) {
		path := f.paths[f.next]
		f.next++
		puller, err := f.env.VectorOpen(f.rctx.Context, f.rctx.Sctx, path, f.format, f.pushdown, len(f.pullers))
		if err != nil {
			if f.env.IgnoreOpenErrors {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			return nil, err
		}
		f.current = puller
		return puller, nil
	}
	return nil, nil
}

type concurrentPuller struct {
	f  *FileScan
	id int

	eos     bool
	current exec.VectorConcurrentPuller
	waitCh  chan struct{}
}

func newPuller(f *FileScan, id int) *concurrentPuller {
	return &concurrentPuller{f: f, id: id, waitCh: make(chan struct{}, 1)}
}

func (p *concurrentPuller) Pull(done bool) (vector.Any, error) {
	if done {
		p.f.done()
		p.eos = true
		p.current = nil
		return nil, nil
	}
	if p.eos {
		p.eos = false
		select {
		case <-p.waitCh:
		case <-p.f.rctx.Context.Done():
			return nil, p.f.rctx.Context.Err()
		}
	}
	for {
		if err := p.f.rctx.Context.Err(); err != nil {
			return nil, err
		}
		if p.current != nil {
			vec, err := p.current.ConcurrentPull(false, p.id)
			if vec != nil || err != nil {
				return vec, err
			}
		}
		puller, err := p.f.nextFile(p.current)
		if err != nil {
			return nil, err
		}
		if puller == nil {
			return p.Pull(true)
		}
		p.current = puller
	}
}
