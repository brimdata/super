package robot

import (
	"fmt"
	"io"
	"net/http"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
)

type Op struct {
	parent   sbuf.Puller
	rctx     *runtime.Context
	env      *exec.Environment
	expr     expr.Evaluator
	pushdown sbuf.Pushdown
	format   string
	batch    sbuf.Batch
	off      int
	src      sbuf.Puller
}

func New(rctx *runtime.Context, env *exec.Environment, parent sbuf.Puller, e expr.Evaluator, format string, p sbuf.Pushdown) *Op {
	return &Op{
		parent:   parent,
		rctx:     rctx,
		env:      env,
		expr:     e,
		pushdown: p,
		format:   format,
	}
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	if done {
		if o.batch != nil {
			o.batch.Unref()
		}
		o.batch = nil
		src := o.src
		o.src = nil
		var err error
		if src != nil {
			var b sbuf.Batch
			b, err = src.Pull(true)
			if b != nil {
				b.Unref()
			}
		}
		b, pullErr := o.parent.Pull(true)
		if b != nil {
			b.Unref()
		}
		if err == nil {
			err = pullErr
		}
		return nil, err
	}
	return o.pullNext()
}

func (o *Op) pullNext() (sbuf.Batch, error) {
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

func (o *Op) getPuller() (sbuf.Puller, error) {
	src, err := o.nextPuller()
	o.src = src
	return src, err
}

func (o *Op) nextPuller() (sbuf.Puller, error) {
	b := o.batch
	if b != nil && o.off >= len(b.Values()) {
		b.Unref()
		o.off = 0
		o.batch = nil
		b = nil
	}
	if b == nil {
		var err error
		b, err = o.nextBatch()
		if err != nil {
			return nil, err
		}
		o.batch = b
		o.off = 0
		if b == nil {
			return nil, nil
		}
	}
	val := b.Values()[o.off]
	o.off++
	target := o.expr.Eval(val)
	if super.TypeUnder(target.Type()) != super.TypeString {
		return o.errOnVal(target), nil
	}
	return o.open(target.AsString())
}

func (o *Op) errOnVal(val super.Value) sbuf.Puller {
	errVal := o.rctx.Sctx.WrapError("from encountered non-string input", val)
	return sbuf.NewPuller(sbuf.NewArray([]super.Value{errVal}))
}

func (o *Op) nextBatch() (sbuf.Batch, error) {
again:
	b, err := o.parent.Pull(false)
	if err != nil {
		return nil, err
	}
	if b == nil {
		o.batch = nil
		o.off = 0
		return nil, nil
	}
	if len(b.Values()) == 0 {
		b.Unref()
		goto again
	}
	o.batch = b
	o.off = 0
	return b, nil
}

func (o *Op) open(path string) (sbuf.Puller, error) {
	u, err := storage.ParseURI(path)
	if err == nil && false {
		//XXX get from AST args, or we can also get this stuff from the
		// robot expr, e.g., allowning the querying to create a record
		// to hold these args...
		var method string
		var body io.Reader
		var headers http.Header
		f, err := o.env.OpenHTTP(o.rctx.Context, o.rctx.Sctx, u.String(), o.format, method, headers, body, nil)
		if err != nil {
			return nil, err
		}
		return f, err
	}
	// This check for attached database will be removed when we add support for pools here.
	if o.env.IsAttached() {
		return nil, fmt.Errorf("%s: cannot open in a database environment", path)
	}
	return o.env.Open(o.rctx.Context, o.rctx.Sctx, path, o.format, o.pushdown)
}
