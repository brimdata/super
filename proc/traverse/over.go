package traverse

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/proc"
	"github.com/brimdata/zed/zbuf"
)

type Over struct {
	exprs  []expr.Evaluator
	parent proc.Interface
	batch  zbuf.Batch
	vals   []zed.Value
	eof    bool
}

func NewOver(parent proc.Interface, exprs []expr.Evaluator) *Over {
	return &Over{
		exprs:  exprs,
		parent: parent,
	}
}

func (o *Over) Pull() (zbuf.Batch, error) {
	if len(o.vals) == 0 {
		batch, err := o.parent.Pull()
		if batch == nil || err != nil {
			return batch, err
		}
		o.eof = false
		o.batch = batch
		o.vals = batch.Values()
	}
	if o.eof {
		o.eof = false
		return nil, nil
	}
	o.eof = true
	out, err := o.over(o.batch.Context(), &o.vals[0])
	o.vals = o.vals[1:]
	if len(o.vals) == 0 {
		o.batch.Unref()
	}
	return out, err
}

// Done is currently ignored as the model here as each downstream batch should be
// handled indepedently.  We need a way to scope flowgraphs so the done protocol can
// be propagated on an outer scope but not on the inner scope.
func (o *Over) Done() {}

func (o *Over) over(ectx expr.Context, this *zed.Value) (*zbuf.Array, error) {
	var vals []zed.Value
	for _, e := range o.exprs {
		val := e.Eval(ectx, this)
		// Propagate errors but skip missing values.
		if !val.IsMissing() {
			var err error
			if vals, err = appendOver(vals, *val); err != nil {
				return nil, err
			}
		}
	}
	return zbuf.NewArray(vals), nil

}

func appendOver(vals []zed.Value, zv zed.Value) ([]zed.Value, error) {
	if zed.IsPrimitiveType(zv.Type) {
		return append(vals, zv), nil
	}
	typ := zed.InnerType(zv.Type)
	if typ == nil {
		// XXX Issue #3324: need to support records and maps.
		return vals, nil
	}
	for it := zv.Bytes.Iter(); !it.Done(); {
		b, _ := it.Next()
		// XXX when we do proper expr.Context, we can allocate
		// this copy through the batch.
		vals = append(vals, *zed.NewValue(typ, b).Copy())
	}
	return vals, nil
}
