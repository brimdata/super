package unnest

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

type Unnest struct {
	parent   zbuf.Puller
	expr     expr.Evaluator
	resetter expr.Resetter

	outer []super.Value
	batch zbuf.Batch
	sctx  *super.Context
}

func NewUnnest(rctx *runtime.Context, parent zbuf.Puller, expr expr.Evaluator, resetter expr.Resetter) *Unnest {
	return &Unnest{
		parent:   parent,
		expr:     expr,
		resetter: resetter,
		sctx:     rctx.Sctx,
	}
}

func (u *Unnest) Pull(done bool) (zbuf.Batch, error) {
	if done {
		u.outer = nil
		u.resetter.Reset()
		return u.parent.Pull(true)
	}
	for {
		if len(u.outer) == 0 {
			batch, err := u.parent.Pull(false)
			if batch == nil || err != nil {
				u.resetter.Reset()
				return nil, err
			}
			u.batch = batch
			u.outer = batch.Values()
		}
		this := u.outer[0]
		u.outer = u.outer[1:]
		innerBatch := u.unnest(this)
		if len(u.outer) == 0 {
			u.batch.Unref()
		}
		if innerBatch != nil {
			return innerBatch, nil
		}
	}
}

func (u *Unnest) unnest(this super.Value) zbuf.Batch {
	val := u.expr.Eval(this)
	// Propagate errors but skip missing values.
	var vals []super.Value
	if !val.IsMissing() {
		vals = unnest(u.sctx, val)
	}
	if len(vals) == 0 {
		return nil
	}
	return zbuf.NewBatch(vals)
}

func unnest(sctx *super.Context, val super.Value) []super.Value {
	val = val.Under()
	switch typ := super.TypeUnder(val.Type()).(type) {
	case *super.TypeArray, *super.TypeSet:
		var vals []super.Value
		typ = super.InnerType(typ)
		for it := val.Bytes().Iter(); !it.Done(); {
			val := super.NewValue(typ, it.Next()).Under()
			vals = append(vals, val.Copy())
		}
		return vals
	case *super.TypeRecord:
		if len(typ.Fields) != 2 {
			return []super.Value{sctx.WrapError("unnest: encountered record without two columns", val)}
		}
		if super.InnerType(typ.Fields[1].Type) == nil {
			return []super.Value{sctx.WrapError("unnest: encountered record without an array column", val)}
		}
		left := *val.DerefByColumn(0)
		fields := slices.Clone(typ.Fields)
		var out []super.Value
		var b zcode.Builder
		for _, right := range unnest(sctx, *val.DerefByColumn(1)) {
			b.Reset()
			b.Append(left.Bytes())
			b.Append(right.Bytes())
			fields[1].Type = right.Type()
			rtyp := sctx.MustLookupTypeRecord(fields)
			out = append(out, super.NewValue(rtyp, b.Bytes()))
		}
		return out
	default:
		return []super.Value{sctx.WrapError("unnest: encountered non-array value", val)}
	}
}
