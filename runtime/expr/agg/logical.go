package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type and struct {
	val *bool
}

func (a *and) Consume(vec vector.Any) {
	if vec.Type().ID() != super.IDBool {
		return
	}
	for i := range vec.Len() {
		if a.val == nil {
			b := true
			a.val = &b
		}
		*a.val = *a.val && vector.BoolValue(vec, i)
	}
}

func (a *and) Result(*super.Context) vector.Any {
	if a.val == nil {
		return vector.NewNull(1)
	}
	if *a.val {
		return vector.NewTrue(1)
	}
	return vector.NewFalse(1)
}

func (a *and) ConsumeAsPartial(partial vector.Any) {
	if kind := partial.Kind(); partial.Len() != 1 || (kind != vector.KindBool && kind != vector.KindNull) {
		panic("and: bad partial")
	}
	a.Consume(partial)
}

func (a *and) ResultAsPartial(*super.Context) vector.Any {
	return a.Result(nil)
}

type or struct {
	val *bool
}

func (o *or) Consume(vec vector.Any) {
	if vec.Type().ID() != super.IDBool {
		return
	}
	for i := range vec.Len() {
		if o.val == nil {
			o.val = new(bool)
		}
		*o.val = *o.val || vector.BoolValue(vec, i)
	}
}

func (o *or) Result(*super.Context) vector.Any {
	if o.val == nil {
		return vector.NewNull(1)
	}
	if *o.val {
		return vector.NewTrue(1)
	}
	return vector.NewFalse(1)
}

func (o *or) ConsumeAsPartial(partial vector.Any) {
	if kind := partial.Kind(); partial.Len() != 1 || (kind != vector.KindBool && kind != vector.KindNull) {
		panic("or: bad partial")
	}
	o.Consume(partial)
}

func (o *or) ResultAsPartial(*super.Context) vector.Any {
	return o.Result(nil)
}
