package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

const maxStackDepth = 10_000

type UDF struct {
	Body       Evaluator
	sctx       *super.Context
	name       string
	fields     []super.Field
	stackDepth *int
	builder    zcode.Builder
}

func NewUDF(sctx *super.Context, name string, params []string, stackDepth *int) *UDF {
	var fields []super.Field
	for _, p := range params {
		fields = append(fields, super.Field{Name: p})
	}
	return &UDF{sctx: sctx, name: name, fields: fields, stackDepth: stackDepth}
}

func (u *UDF) Call(ectx super.Allocator, args []super.Value) super.Value {
	*u.stackDepth++
	if *u.stackDepth > maxStackDepth {
		return u.sctx.NewErrorf("stack overflow in function %q", u.name)
	}
	defer func() { *u.stackDepth-- }()
	if len(args) == 0 {
		return u.Body.Eval(NewContext(), super.Null)
	}
	u.builder.Reset()
	for i, a := range args {
		u.fields[i].Type = a.Type()
		u.builder.Append(a.Bytes())
	}
	typ := u.sctx.MustLookupTypeRecord(u.fields)
	return u.Body.Eval(NewContext(), super.NewValue(typ, u.builder.Bytes()))
}
