package expr

import (
	"fmt"

	"github.com/brimdata/super"
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
// XXX rename this and add structured error info... this now checks for error first
// and returns the error, then checks for null
func CheckForNullThenError(sctx *super.Context, vecs []vector.Any, msg string) (vector.Any, bool) {
	for _, vec := range vecs {
		if vec.Kind() == vector.KindError {
			return vector.NewWrappedError(sctx, fmt.Sprintf("%s: error value encountered", msg), vec), true
		}
	}
	for _, vec := range vecs {
		if vec.Kind() == vector.KindNull {
			return vec, true
		}
	}
	return nil, false
}

type Call struct {
	fn       Function
	defuse   *Defuse
	exprs    []Evaluator
	applyOpt vector.ApplyOpt
	args     []vector.Any
}

func NewCall(sctx *super.Context, fn Function, exprs []Evaluator) *Call {
	opt := vector.ApplyRipUnions
	if fn, ok := fn.(interface{ ApplyOpt() vector.ApplyOpt }); ok {
		opt = fn.ApplyOpt()
	}
	defuse := NewDefuse(sctx)
	if fn, ok := fn.(interface{ NoDefuse() bool }); ok {
		if fn.NoDefuse() {
			defuse = nil
		}
	}
	return &Call{
		fn:       fn,
		defuse:   defuse,
		exprs:    exprs,
		applyOpt: opt,
		args:     make([]vector.Any, len(exprs)),
	}
}

func (c *Call) Eval(this vector.Any) vector.Any {
	for k, e := range c.exprs {
		vec := e.Eval(this)
		if c.defuse != nil {
			vec = c.defuse.Eval(vec)
		}
		c.args[k] = vec
	}
	return vector.Apply(c.applyOpt, c.fn.Call, c.args...)
}
