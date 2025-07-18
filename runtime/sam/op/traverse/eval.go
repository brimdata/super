package traverse

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

// Expr provides provides glue to run a traversal subquery in expression
// context.  It implements zbuf.Puller so it can serve as the data source
// to the subquery as well as expr.Evalulator so it can be called from an
// expression.  Each time its Eval method is called, it propagates the value
// to the batch channel to be pulled into the scope.  If there is
// just one result, then the value is returned.  If there are multiple results
// then they are returned in an array (with union elements if the type varies).
type Expr struct {
	ctx     context.Context
	sctx    *super.Context
	batchCh chan zbuf.Batch
	eos     bool

	exit *Exit
	out  []zbuf.Batch
}

var _ expr.Evaluator = (*Expr)(nil)
var _ zbuf.Puller = (*Expr)(nil)

func NewExpr(ctx context.Context, sctx *super.Context) *Expr {
	return &Expr{
		ctx:     ctx,
		sctx:    sctx,
		batchCh: make(chan zbuf.Batch, 1),
	}
}

func (e *Expr) SetExit(exit *Exit) {
	e.exit = exit
}

func (e *Expr) Eval(this super.Value) super.Value {
	b := zbuf.NewArray([]super.Value{this})
	select {
	case e.batchCh <- b:
	case <-e.ctx.Done():
		return e.sctx.NewError(e.ctx.Err())
	}
	out := e.out[:0]
	for {
		b, err := e.exit.Pull(false)
		if err != nil {
			panic(err)
		}
		if b == nil {
			e.out = out
			return e.combine(out)
		}
		out = append(out, b)
	}
}

func (e *Expr) combine(batches []zbuf.Batch) super.Value {
	switch len(batches) {
	case 0:
		return super.Null
	case 1:
		return e.makeArray(batches[0].Values())
	default:
		var vals []super.Value
		for _, batch := range batches {
			vals = append(vals, batch.Values()...)
		}
		return e.makeArray(vals)
	}
}

func (e *Expr) makeArray(vals []super.Value) super.Value {
	if len(vals) == 0 {
		return super.Null
	}
	if len(vals) == 1 {
		return vals[0]
	}
	typ := vals[0].Type()
	for _, val := range vals[1:] {
		if typ != val.Type() {
			return e.makeUnionArray(vals)
		}
	}
	var b zcode.Builder
	for _, val := range vals {
		b.Append(val.Bytes())
	}
	return super.NewValue(e.sctx.LookupTypeArray(typ), b.Bytes())
}

func (e *Expr) makeUnionArray(vals []super.Value) super.Value {
	types := make(map[super.Type]struct{})
	for _, val := range vals {
		types[val.Type()] = struct{}{}
	}
	utypes := make([]super.Type, 0, len(types))
	for typ := range types {
		utypes = append(utypes, typ)
	}
	union := e.sctx.LookupTypeUnion(utypes)
	var b zcode.Builder
	for _, val := range vals {
		super.BuildUnion(&b, union.TagOf(val.Type()), val.Bytes())
	}
	return super.NewValue(e.sctx.LookupTypeArray(union), b.Bytes())
}

func (e *Expr) Pull(done bool) (zbuf.Batch, error) {
	if e.eos {
		e.eos = false
		return nil, nil
	}
	e.eos = true
	select {
	case batch := <-e.batchCh:
		return batch, nil
	case <-e.ctx.Done():
		return nil, e.ctx.Err()
	}
}
