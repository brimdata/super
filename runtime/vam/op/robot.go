package op

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
)

type Robot struct {
	parent   vector.Puller
	rctx     *runtime.Context
	env      *exec.Environment
	expr     expr.Evaluator
	pushdown sbuf.Pushdown
	format   string
	vecs     []vector.Any
	vec      vector.Any
	off      uint32
	src      vector.Puller
}

func NewRobot(rctx *runtime.Context, env *exec.Environment, parent vector.Puller, e expr.Evaluator, format string, p sbuf.Pushdown) *Robot {
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
		o.vecs = nil
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

func (o *Robot) getPuller() (vector.Puller, error) {
	src, err := o.nextPuller()
	o.src = src
	return src, err
}

func (o *Robot) nextPuller() (vector.Puller, error) {
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
	if vec.Type().ID() != super.IDString {
		return o.errOnVal(vec), nil
	}
	for {
		s, null := vector.StringValue(vec, o.off)
		o.off++
		if null {
			continue
		}
		return o.open(s)
	}
}

func (o *Robot) errOnVal(vec vector.Any) vector.Puller {
	out := vector.NewWrappedError(o.rctx.Sctx, "from ecountered non-string input", vec)
	return vector.NewPuller(out)
}

func (o *Robot) nextVec() (vector.Any, error) {
	if len(o.vecs) == 0 {
	again:
		in, err := o.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if in == nil {
			o.vec = nil
			o.off = 0
			return nil, nil
		}
		if in.Len() == 0 {
			goto again
		}
		in = o.expr.Eval(in)
		o.vecs = []vector.Any{in}
		if d, ok := in.(*vector.Dynamic); ok {
			o.vecs = d.Values
		}
	}
	vec := o.vecs[0]
	o.vec = vec
	o.off = 0
	o.vecs = o.vecs[1:]
	return vec, nil
}

func (o *Robot) open(path string) (vector.Puller, error) {
	// This check for attached database will be removed when we add support for pools here.
	if o.env.IsAttached() {
		return nil, fmt.Errorf("%s: cannot open in a database environment", path)
	}
	return o.env.VectorOpen(o.rctx.Context, o.rctx.Sctx, path, o.format, o.pushdown, 1)
}
