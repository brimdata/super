package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

const maxStackDepth = 10_000

type UDF struct {
	Body    Evaluator
	sctx    *super.Context
	name    string
	fields  []super.Field
	builder zcode.Builder
}

func NewUDF(sctx *super.Context, name string, params []string) *UDF {
	var fields []super.Field
	for _, p := range params {
		fields = append(fields, super.Field{Name: p})
	}
	return &UDF{sctx: sctx, name: name, fields: fields}
}

func (u *UDF) Call(ectx super.Allocator, args []super.Value) super.Value {
	f, ok := ectx.(*frame)
	if ok {
		f.stack++
	} else {
		f = &frame{1}
	}
	if f.stack > maxStackDepth {
		return u.sctx.NewErrorf("stack overflow in function %q", u.name)
	}
	defer f.exit()
	if len(args) == 0 {
		return u.Body.Eval(f, super.Null)
	}
	u.builder.Reset()
	for i := range args {
		u.fields[i].Type = args[i].Type()
		u.builder.Append(args[i].Bytes())
	}
	typ := u.sctx.MustLookupTypeRecord(u.fields)
	return u.Body.Eval(f, super.NewValue(typ, u.builder.Bytes()))
}

type frame struct {
	stack int
}

func (f *frame) Vars() []super.Value {
	return nil
}

func (f *frame) exit() {
	f.stack--
}
