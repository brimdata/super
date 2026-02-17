package expr

import (
	"github.com/brimdata/super/vector"
)

type Evaluator interface {
	Eval(vector.Any) vector.Any
}

type Function interface {
	Call(...vector.Any) vector.Any
}

// CheckForNullThenError returns the first element of vecs with the null
// type. If no element has the null type, it returns the first element with an
// error type.
func CheckForNullThenError(vecs []vector.Any) (vector.Any, bool) {
	var errVec vector.Any
	for _, vec := range vecs {
		if k := vec.Kind(); k == vector.KindNull {
			return vec, true
		} else if k == vector.KindError && errVec == nil {
			errVec = vec
		}
	}
	return errVec, errVec != nil
}

type Call struct {
	fn        Function
	exprs     []Evaluator
	ripUnions bool
	args      []vector.Any
}

func NewCall(fn Function, exprs []Evaluator) *Call {
	ripUnions := true
	if fn, ok := fn.(interface{ RipUnions() bool }); ok {
		ripUnions = fn.RipUnions()
	}
	return &Call{
		fn:        fn,
		exprs:     exprs,
		ripUnions: ripUnions,
		args:      make([]vector.Any, len(exprs)),
	}
}

func (c *Call) Eval(this vector.Any) vector.Any {
	for k, e := range c.exprs {
		c.args[k] = e.Eval(this)
	}
	return vector.Apply(c.ripUnions, c.fn.Call, c.args...)
}
