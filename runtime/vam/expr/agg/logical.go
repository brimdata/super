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
		v, isnull := vector.BoolValue(vec, i)
		if isnull {
			continue
		}
		if a.val == nil {
			b := true
			a.val = &b
		}
		*a.val = *a.val && v
	}
}

func (a *and) Result(*super.Context) super.Value {
	if a.val == nil {
		return super.NullBool
	}
	return super.NewBool(*a.val)
}

func (a *and) ConsumeAsPartial(partial vector.Any) {
	if partial.Len() != 1 || partial.Type().ID() != super.IDBool {
		panic("and: bad partial")
	}
	a.Consume(partial)
}

func (a *and) ResultAsPartial(*super.Context) super.Value {
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
		v, isnull := vector.BoolValue(vec, i)
		if isnull {
			continue
		}
		if o.val == nil {
			b := false
			o.val = &b
		}
		*o.val = *o.val || v
	}
}

func (o *or) Result(*super.Context) super.Value {
	if o.val == nil {
		return super.NullBool
	}
	return super.NewBool(*o.val)
}

func (o *or) ConsumeAsPartial(partial vector.Any) {
	if partial.Len() != 1 || partial.Type().ID() != super.IDBool {
		panic("or: bad partial")
	}
	o.Consume(partial)
}

func (o *or) ResultAsPartial(*super.Context) super.Value {
	return o.Result(nil)
}
