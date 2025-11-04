package filescan

import (
	"fmt"
	"os"

	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sbuf"
)

type fileScan struct {
	rctx     *runtime.Context
	env      *exec.Environment
	paths    []string
	format   string
	pushdown sbuf.Pushdown

	next   int
	puller sbuf.Puller
}

func New(rctx *runtime.Context, env *exec.Environment, paths []string, format string, p sbuf.Pushdown) sbuf.Puller {
	return &fileScan{
		rctx:     rctx,
		env:      env,
		paths:    paths,
		format:   format,
		pushdown: p,
	}
}

func (f *fileScan) Pull(done bool) (sbuf.Batch, error) {
	if done {
		var err error
		if f.puller != nil {
			_, err = f.puller.Pull(true)
		}
		// Prepare to restart.
		f.next = 0
		f.puller = nil
		return nil, err
	}
	for {
		if f.puller == nil {
			puller, err := f.openNext()
			if puller == nil || err != nil {
				// Prepare to restart.
				f.next = 0
				return nil, err
			}
			f.puller = puller
		}
		batch, err := f.puller.Pull(false)
		if batch != nil || err != nil {
			return batch, err
		}
		f.puller = nil
	}
}

func (f *fileScan) openNext() (sbuf.Puller, error) {
	for f.next < len(f.paths) {
		path := f.paths[f.next]
		f.next++
		puller, err := f.env.Open(f.rctx.Context, f.rctx.Sctx, path, f.format, f.pushdown)
		if err != nil {
			if f.env.IgnoreOpenErrors {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			return nil, err
		}
		return puller, nil
	}
	return nil, nil
}
