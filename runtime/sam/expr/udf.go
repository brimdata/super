package expr

import (
	"slices"

	"github.com/brimdata/super"
)

const maxStackDepth = 10_000

type UDF struct {
	Body Evaluator
}

func (u *UDF) Call(ectx super.Allocator, args []super.Value) super.Value {
	stack := 1
	if f, ok := ectx.(*frame); ok {
		stack += f.stack
	}
	if stack > maxStackDepth {
		panic("stack overflow")
	}
	// args must be cloned otherwise the values will be overwritten in
	// recursive calls.
	f := &frame{stack: stack, vars: slices.Clone(args)}
	defer f.exit()
	return u.Body.Eval(f, super.Null)
}

type frame struct {
	allocator
	stack int
	vars  []super.Value
}

func (f *frame) Vars() []super.Value {
	return f.vars
}

func (f *frame) exit() {
	f.stack--
}
