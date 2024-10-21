package agg

import (
	"github.com/brimdata/super"
)

type And struct {
	val *bool
}

var _ Function = (*And)(nil)

func (a *And) Consume(val super.Value) {
	if val.IsNull() || super.TypeUnder(val.Type()) != super.TypeBool {
		return
	}
	if a.val == nil {
		b := true
		a.val = &b
	}
	*a.val = *a.val && val.Bool()
}

func (a *And) Result(*super.Context) super.Value {
	if a.val == nil {
		return super.NullBool
	}
	return super.NewBool(*a.val)
}

func (a *And) ConsumeAsPartial(val super.Value) {
	if val.Type() != super.TypeBool {
		panic("and: partial not a bool")
	}
	a.Consume(val)
}

func (a *And) ResultAsPartial(*super.Context) super.Value {
	return a.Result(nil)
}

type Or struct {
	val *bool
}

var _ Function = (*Or)(nil)

func (o *Or) Consume(val super.Value) {
	if val.IsNull() || super.TypeUnder(val.Type()) != super.TypeBool {
		return
	}
	if o.val == nil {
		b := false
		o.val = &b
	}
	*o.val = *o.val || val.Bool()
}

func (o *Or) Result(*super.Context) super.Value {
	if o.val == nil {
		return super.NullBool
	}
	return super.NewBool(*o.val)
}

func (o *Or) ConsumeAsPartial(val super.Value) {
	if val.Type() != super.TypeBool {
		panic("or: partial not a bool")
	}
	o.Consume(val)
}

func (o *Or) ResultAsPartial(*super.Context) super.Value {
	return o.Result(nil)
}
