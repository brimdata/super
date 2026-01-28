package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Values struct {
	sctx   *super.Context
	parent vector.Puller
	exprs  []expr.Evaluator
}

var _ vector.Puller = (*Values)(nil)

func NewValues(sctx *super.Context, parent vector.Puller, exprs []expr.Evaluator) *Values {
	return &Values{
		sctx:   sctx,
		parent: parent,
		exprs:  exprs,
	}
}

func (v *Values) Pull(done bool) (vector.Any, error) {
	for {
		val, err := v.parent.Pull(done)
		if val == nil {
			return nil, err
		}
		vals := make([]vector.Any, 0, len(v.exprs))
		for _, e := range v.exprs {
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
		mask, hasfiltered := expr.QuietMask(vecs[0])
		filtered = filtered || hasfiltered
		return mask
	}, vec)
	if !filtered {
		return vec
	}
	masked, _ := applyMask(vec, mask)
	return masked
}
