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

func CheckNulls(vecs []vector.Any) (vector.Any, bool) {
	for _, vec := range vecs {
		if vec.Kind() == vector.KindNull {
			return vec, true
		}
	}
	return nil, false
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
