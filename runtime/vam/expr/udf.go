package expr

import (
	"fmt"
	"slices"

	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type UDF struct {
	Body       Evaluator
	sctx       *super.Context
	name       string
	fields     []super.Field
	stackDepth int
}

func NewUDF(sctx *super.Context, name string, params []string) *UDF {
	var fields []super.Field
	for _, p := range params {
		fields = append(fields, super.Field{Name: p})
	}
	return &UDF{sctx: sctx, name: name, fields: fields}
}

func (u *UDF) Call(args ...vector.Any) vector.Any {
	u.stackDepth++
	if u.stackDepth > samexpr.MaxStackDepth {
		return vector.NewStringError(u.sctx, fmt.Sprintf("stack overflow in function %q", u.name), args[0].Len())
	}
	defer func() { u.stackDepth-- }()
	if len(u.fields) == 0 {
		return u.Body.Eval(vector.NewConst(super.Null, args[0].Len(), bitvec.Zero))
	}
	fields := slices.Clone(u.fields)
	for i := range args {
		fields[i].Type = args[i].Type()
	}
	typ := u.sctx.MustLookupTypeRecord(fields)
	vec := vector.NewRecord(typ, slices.Clone(args), args[0].Len(), bitvec.Zero)
	return u.Body.Eval(vec)
}
