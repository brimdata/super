package traverse

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

type Over struct {
	parent   zbuf.Puller
	exprs    []expr.Evaluator
	resetter expr.Resetter

	outer []super.Value
	batch zbuf.Batch
	enter *Enter
	zctx  *super.Context
}

func NewOver(rctx *runtime.Context, parent zbuf.Puller, exprs []expr.Evaluator, resetter expr.Resetter) *Over {
	return &Over{
		parent:   parent,
		exprs:    exprs,
		resetter: resetter,
		zctx:     rctx.Zctx,
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
		o.resetter.Reset()
		return o.parent.Pull(true)
	}
	for {
		if len(o.outer) == 0 {
			batch, err := o.parent.Pull(false)
			if batch == nil || err != nil {
				o.resetter.Reset()
				return nil, err
			}
			o.batch = batch
			o.outer = batch.Values()
		}
		this := o.outer[0]
		o.outer = o.outer[1:]
		ectx := o.batch
		if o.enter != nil {
			ectx = o.enter.addLocals(ectx, this)
		}
		innerBatch := o.over(ectx, this)
		if len(o.outer) == 0 {
			o.batch.Unref()
		}
		if innerBatch != nil {
			return innerBatch, nil
		}
	}
}

func (o *Over) over(batch zbuf.Batch, this super.Value) zbuf.Batch {
	// Copy the vars into a new scope since downstream, nested subgraphs
	// can have concurrent operators.  We can optimize these copies out
	// later depending on the nested subgraph.
	var vals []super.Value
	for _, e := range o.exprs {
		val := e.Eval(batch, this)
		// Propagate errors but skip missing values.
		if !val.IsMissing() {
			vals = appendOver(o.zctx, vals, val)
		}
	}
	if len(vals) == 0 {
		return nil
	}
	return zbuf.NewBatch(batch, vals)
}

func appendOver(zctx *super.Context, vals []super.Value, val super.Value) []super.Value {
	val = val.Under()
	switch typ := super.TypeUnder(val.Type()).(type) {
	case *super.TypeArray, *super.TypeSet:
		typ = super.InnerType(typ)
		for it := val.Bytes().Iter(); !it.Done(); {
			// XXX when we do proper expr.Context, we can allocate
			// this copy through the batch.
			val := super.NewValue(typ, it.Next())
			val = val.Under()
			vals = append(vals, val.Copy())
		}
		return vals
	case *super.TypeMap:
		rtyp := zctx.MustLookupTypeRecord([]super.Field{
			super.NewField("key", typ.KeyType),
			super.NewField("value", typ.ValType),
		})
		for it := val.Bytes().Iter(); !it.Done(); {
			bytes := zcode.Append(zcode.Append(nil, it.Next()), it.Next())
			vals = append(vals, super.NewValue(rtyp, bytes))
		}
		return vals
	case *super.TypeRecord:
		builder := zcode.NewBuilder()
		for i, it := 0, val.Bytes().Iter(); !it.Done(); i++ {
			builder.Reset()
			field := typ.Fields[i]
			typ := zctx.MustLookupTypeRecord([]super.Field{
				{Name: "key", Type: zctx.LookupTypeArray(super.TypeString)},
				{Name: "value", Type: field.Type},
			})
			builder.BeginContainer()
			builder.Append(super.EncodeString(field.Name))
			builder.EndContainer()
			builder.Append(it.Next())
			vals = append(vals, super.NewValue(typ, builder.Bytes()).Copy())
		}
		return vals
	default:
		return append(vals, val.Copy())
	}
}
