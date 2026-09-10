package expr

import (
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type This struct{}

func (*This) Eval(val vector.Any) vector.Any {
	return val
}

type DotExpr struct {
	sctx    *super.Context
	defuse  *Defuse
	entity  Evaluator
	key     string
	noneish bool
	okPush  bool
}

func NewDotExpr(sctx *super.Context, record Evaluator, field string, noneish bool) *DotExpr {
	return &DotExpr{
		sctx:    sctx,
		defuse:  NewDefuse(sctx),
		entity:  record,
		key:     field,
		noneish: noneish,
	}
}

func NewDottedExpr(sctx *super.Context, f field.Chain) Evaluator {
	ret := Evaluator(&This{})
	for _, elem := range f {
		ret = NewDotExpr(sctx, ret, elem.ID, elem.Noneish)
	}
	return ret
}

func (d *DotExpr) Eval(vec vector.Any) vector.Any {
	return vector.Apply(vector.ApplyNone, d.eval, d.entity.Eval(vec))
}

func (d *DotExpr) eval(outerVecs ...vector.Any) vector.Any {
	vec := outerVecs[0]
	var missing bool
	eval := func(innerVecs ...vector.Any) vector.Any {
		switch val := vector.Under(innerVecs[0]).(type) {
		case *vector.None:
			return val
		case *vector.Record:
			i, ok := val.Typ.IndexOfField(d.key)
			if !ok {
				missing = true
				return vector.NewWrappedError(d.sctx, fmt.Sprintf("no such field %s", sup.QuotedName(d.key)), innerVecs[0])
			}
			out := val.Fields[i]
			if hasNone(out) {
				missing = true
			}
			return out
		case *vector.TypeValue:
			var errs []uint32
			typvals := vector.NewTypeValueEmpty()
			for i := range val.Len() {
				typ := val.Value(i)
				if typ, ok := super.TypeUnder(typ).(*super.TypeRecord); ok {
					if typ, ok := typ.TypeOfField(d.key); ok {
						typvals.Append(typ)
						continue
					}
				}
				errs = append(errs, i)
			}
			if len(errs) > 0 {
				return vector.NewCombinedError(d.sctx, fmt.Sprintf("no such field %s", sup.QuotedName(d.key)), typvals, val, errs)
			}
			return typvals
		case *vector.Map:
			keyVec := vector.NewConstString(d.key, val.Len())
			return indexMap(d.sctx, val, keyVec)
		case *vector.View:
			return vector.Pick(d.eval(val.Any), val.Index)
		default:
			dot := "."
			if d.noneish {
				dot = "?."
			}
			return vector.NewWrappedError(d.sctx, fmt.Sprintf("'%s': applied to non-record", dot), innerVecs[0])
		}
	}
	out := vector.Apply(vector.ApplyRipFusions|vector.ApplyRipUnions, eval, vec)
	// If there were any structured errors or none values (e.g., because we hit a none
	// inside a fusion and thus should be an error), then we take the slow path
	// by defusing and starting over.  One simple optimization we can do is okPush
	// to indicate that this reference is wrapped in an ok() or is_ok(), in which case,
	// the on field of the structured error will be discarded and thus does not need
	// to be correct.  There are a number of other ways to avoid this slow path but let's
	// get it working first before we make it fast.
	// XXX we need to wire up okPush
	if !d.okPush && missing && vec.Kind() == vector.KindFusion {
		return vector.Apply(vector.ApplyRipFusions|vector.ApplyRipUnions, d.eval, d.defuse.Eval(vec))
	}
	return out
}

func hasNone(vec vector.Any) bool {
	if _, ok := vec.(*vector.None); ok {
		return true
	}
	if vec, ok := vec.(*vector.Fusion); ok {
		return hasNone(vec.Values)
	}
	if vec, ok := vec.(*vector.Dynamic); ok {
		return slices.IndexFunc(vec.Values, hasNone) >= 0
	}
	if super.IsOptionType(vec.Type()) {
		return hasNone(vec.(*vector.Union).Dynamic())
	}
	return false
}
