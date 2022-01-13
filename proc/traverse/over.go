package traverse

import (
	"context"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/proc"
	"github.com/brimdata/zed/zbuf"
)

type Over struct {
	parent zbuf.Puller
	exprs  []expr.Evaluator
	outer  []zed.Value
	batch  zbuf.Batch
	enter  *Enter
}

func NewOver(pctx *proc.Context, parent zbuf.Puller, exprs []expr.Evaluator) *Over {
	return &Over{
		parent: parent,
		exprs:  exprs,
	}
}

func (o *Over) AddScope(ctx context.Context, names []string, exprs []expr.Evaluator) *Scope {
	scope := newScope(ctx, o, names, exprs)
	o.enter = scope.enter
	return scope
}

func (o *Over) Pull(done bool) (zbuf.Batch, error) {
	if done {
		o.outer = nil
		return o.parent.Pull(true)
	}
	if len(o.outer) == 0 {
		batch, err := o.parent.Pull(false)
		if batch == nil || err != nil {
			return nil, err
		}
		o.batch = batch
		o.outer = batch.Values()
	}
	this := &o.outer[0]
	o.outer = o.outer[1:]
	ectx := o.batch
	if o.enter != nil {
		ectx = o.enter.addLocals(ectx, this)
	}
	innerBatch := o.over(ectx, this)
	if len(o.outer) == 0 {
		o.batch.Unref()
	}
	return innerBatch, nil
}

func (o *Over) over(batch zbuf.Batch, this *zed.Value) zbuf.Batch {
	// Copy the vars into a new scope since downstream, nested subgraphs
	// can have concurrent operators.  We can optimize these copies out
	// later depending on the nested subgraph.
	var vals []zed.Value
	for _, e := range o.exprs {
		val := e.Eval(batch, this)
		// Propagate errors but skip missing values.
		if !val.IsMissing() {
			vals = appendOver(vals, *val)
		}
	}
	return zbuf.NewBatch(batch, vals)
}

func appendOver(vals []zed.Value, zv zed.Value) []zed.Value {
	if zed.IsPrimitiveType(zv.Type) {
		return append(vals, zv)
	}
	typ := zed.InnerType(zv.Type)
	if typ == nil {
		// XXX Issue #3324: need to support records and maps.
		return vals
	}
	for it := zv.Bytes.Iter(); !it.Done(); {
		b, _ := it.Next()
		// XXX when we do proper expr.Context, we can allocate
		// this copy through the batch.
		vals = append(vals, *zed.NewValue(typ, b).Copy())
	}
	return vals
}
