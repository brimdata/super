package op

import (
	"fmt"

	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Robot struct {
	parent   vio.Puller
	rctx     *runtime.Context
	env      *exec.Environment
	expr     expr.Evaluator
	pushdown sbuf.Pushdown
	format   string
	vec      vector.Any
	off      uint32
	src      vio.Puller
}

func NewRobot(rctx *runtime.Context, env *exec.Environment, parent vio.Puller, e expr.Evaluator, format string, p sbuf.Pushdown) *Robot {
	return &Robot{
		parent:   parent,
		rctx:     rctx,
		env:      env,
		expr:     e,
		pushdown: p,
		format:   format,
	}
}

func (o *Robot) Pull(done bool) (vector.Any, error) {
	if done {
		o.off = 0
		o.vec = nil
		src := o.src
		o.src = nil
		var err error
		if src != nil {
			_, err = src.Pull(true)
		}
		if _, pullErr := o.parent.Pull(true); err == nil {
			err = pullErr
		}
		return nil, err
	}
	return o.pullNext()
}

func (o *Robot) pullNext() (vector.Any, error) {
	for {
		puller := o.src
		if puller == nil {
			var err error
			puller, err = o.getPuller()
			if puller == nil || err != nil {
				return nil, err
			}
		}
		b, err := puller.Pull(false)
		if b != nil {
			return b, err
		}
		o.src = nil
		if err != nil {
			return nil, err
		}
		_, err = puller.Pull(true)
		if err != nil {
			return nil, err
		}
	}
}

func (o *Robot) getPuller() (vio.Puller, error) {
	src, err := o.nextPuller()
	o.src = src
	return src, err
}

func (o *Robot) nextPuller() (vio.Puller, error) {
	vec := o.vec
	if vec != nil && o.off >= vec.Len() {
		o.off = 0
		o.vec = nil
		vec = nil
	}
	if vec == nil {
		var err error
		if vec, err = o.nextVec(); err != nil {
			return nil, err
		}
		o.vec = vec
		o.off = 0
		if vec == nil {
			return nil, nil
		}
	}
	off := o.off
	o.off++
	var b scode.Builder
	val := vector.ValueAt(&b, vec, off)
	if !val.IsString() {
		return o.errOnVal(vector.Pick(vec, []uint32{off})), nil
	}
	return o.open(val.AsString())
}

func (o *Robot) errOnVal(vec vector.Any) vio.Puller {
	out := vector.NewWrappedError(o.rctx.Sctx, "from ecountered non-string input", vec)
	return vio.NewPuller(out)
}

func (o *Robot) nextVec() (vector.Any, error) {
	vec, err := o.parent.Pull(false)
	if err != nil {
		return nil, err
	}
	if vec == nil {
		o.vec = nil
		o.off = 0
		return nil, nil
	}
	vec = o.expr.Eval(vec)
	o.vec = vec
	o.off = 0
	return vec, nil
}

func (o *Robot) open(path string) (vio.Puller, error) {
	// This check for attached database will be removed when we add support for pools here.
	if o.env.IsAttached() {
		return nil, fmt.Errorf("%s: cannot open in a database environment", path)
	}
	return o.env.VectorOpen(o.rctx.Context, o.rctx.Sctx, path, o.format, o.pushdown, 1)
}
