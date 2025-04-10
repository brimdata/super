package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Yield struct {
	sctx   *super.Context
	parent vector.Puller
	exprs  []expr.Evaluator
}

var _ vector.Puller = (*Yield)(nil)

func NewYield(sctx *super.Context, parent vector.Puller, exprs []expr.Evaluator) *Yield {
	return &Yield{
		sctx:   sctx,
		parent: parent,
		exprs:  exprs,
	}
}

func (y *Yield) Pull(done bool) (vector.Any, error) {
	for {
		val, err := y.parent.Pull(done)
		if val == nil {
			return nil, err
		}
		vals := make([]vector.Any, 0, len(y.exprs))
		for _, e := range y.exprs {
			v := filterQuiet(e.Eval(val))
			if v != nil {
				vals = append(vals, v)
			}
		}
		if len(vals) == 1 {
			return vals[0], nil
		} else if len(vals) != 0 {
			return interleave(vals), nil
		}
		// If no vals, continue the loop.
	}
}

// XXX should work for vector.Dynamic
func interleave(vals []vector.Any) vector.Any {
	if len(vals) < 2 {
		panic("interleave requires two or more vals")
	}
	n := vals[0].Len()
	nvals := uint32(len(vals))
	tags := make([]uint32, n*nvals)
	for k := uint32(0); k < n*nvals; k++ {
		tags[k] = k % nvals

	}
	return vector.NewDynamic(tags, vals)
}

func filterQuiet(vec vector.Any) vector.Any {
	var filtered bool
	mask := vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		mask, hasfiltered := quietMask(vecs[0])
		filtered = filtered || hasfiltered
		return mask
	}, vec)
	if !filtered {
		return vec
	}
	masked, _ := applyMask(vec, mask)
	return masked
}

func quietMask(vec vector.Any) (vector.Any, bool) {
	errvec, ok := vec.(*vector.Error)
	if !ok {
		return vector.NewConst(super.True, vec.Len(), bitvec.Zero), false
	}
	if _, ok := errvec.Vals.Type().(*super.TypeOfString); !ok {
		return vector.NewConst(super.True, vec.Len(), bitvec.Zero), false
	}
	if c, ok := errvec.Vals.(*vector.Const); ok {
		if s, _ := c.AsString(); s == "quiet" {
			return vector.NewConst(super.False, vec.Len(), bitvec.Zero), true
		}
		return vector.NewConst(super.True, vec.Len(), bitvec.Zero), false
	}
	n := vec.Len()
	mask := vector.NewFalse(n)
	switch vec := vec.(type) {
	case *vector.Error:
		for i := uint32(0); i < n; i++ {
			if s, _ := vector.StringValue(vec.Vals, i); s == "quiet" {
				continue
			}
			mask.Set(i)
		}
	}
	return mask, true
}
