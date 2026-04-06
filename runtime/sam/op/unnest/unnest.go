package unnest

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
)

type Unnest struct {
	parent sbuf.Puller
	expr   expr.Evaluator

	outer []super.Value
	batch sbuf.Batch
	sctx  *super.Context
}

func NewUnnest(rctx *runtime.Context, parent sbuf.Puller, expr expr.Evaluator) *Unnest {
	return &Unnest{
		parent: parent,
		expr:   expr,
		sctx:   rctx.Sctx,
	}
}

func (u *Unnest) Pull(done bool) (sbuf.Batch, error) {
	if done {
		u.outer = nil
		return u.parent.Pull(true)
	}
	for {
		if len(u.outer) == 0 {
			batch, err := u.parent.Pull(false)
			if batch == nil || err != nil {
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

func (u *Unnest) unnest(this super.Value) sbuf.Batch {
	val := u.expr.Eval(this).DeunionIntoNameds()
	// Propagate errors but skip missing values.
	var vals []super.Value
	if !val.IsMissing() {
		vals = unnest(u.sctx, val)
	}
	if len(vals) == 0 {
		return nil
	}
	return sbuf.NewBatch(vals)
}

func unnest(sctx *super.Context, val super.Value) []super.Value {
	val = val.Under()
	switch typ := super.TypeUnder(val.Type()).(type) {
	case *super.TypeArray, *super.TypeSet:
		return unnestArrayOrSet(sctx, val)
	case *super.TypeRecord:
		if len(typ.Fields) != 2 {
			return []super.Value{sctx.WrapError("unnest: encountered record without two fields", val)}
		}
		it := scode.NewRecordIter(val.Bytes(), typ.Opts)
		left, none := it.Next(typ.Fields[0].Opt)
		if none {
			return nil
		}
		right, none := it.Next(typ.Fields[1].Opt)
		if none {
			return nil
		}
		fields := slices.Clone(typ.Fields)
		var out []super.Value
		var b scode.Builder
		for _, elem := range unnestArrayOrSet(sctx, super.NewValue(typ.Fields[1].Type, right).Under()) {
			b.Reset()
			b.Append(left)
			b.Append(elem.Bytes())
			fields[1].Type = elem.Type()
			rtyp := sctx.MustLookupTypeRecord(fields)
			out = append(out, super.NewValue(rtyp, b.Bytes()))
		}
		return out
	default:
		if val.IsNull() {
			return nil
		}
		return []super.Value{sctx.WrapError("unnest: encountered non-array value", val)}
	}
}

func unnestArrayOrSet(sctx *super.Context, val super.Value) []super.Value {
	elemType := super.InnerType(val.Type())
	if elemType == nil {
		return []super.Value{sctx.WrapError("unnest: encountered record without an array/set type for second field", val)}
	}
	var vals []super.Value
	for it := val.Bytes().Iter(); !it.Done(); {
		val := super.NewValue(elemType, it.Next()).Under()
		vals = append(vals, val.Copy())
	}
	return vals
}
