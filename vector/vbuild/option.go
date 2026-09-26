package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type optionBuilder struct {
	typ    *super.TypeOption
	values Builder
	tags   []uint32
	nones  uint32
}

func newOptionBuilder(typ *super.TypeOption) Builder {
	return &optionBuilder{typ: typ, values: New(typ.Type)}
}

func (o *optionBuilder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	option := vector.PushView(vec).(*vector.Option)
	option.Apply(func(tags []uint32, some vector.Any, none *vector.None) vector.Any {
		if some == nil {
			n := none.Len()
			for range n {
				o.tags = append(o.tags, vector.OptionNoneTag)
			}
			o.nones += n
			return nil
		}
		if none == nil {
			o.values.Write(some)
			for range some.Len() {
				o.tags = append(o.tags, vector.OptionSomeTag)
			}
			return nil
		}
		// both style
		for _, tag := range tags {
			o.tags = append(o.tags, tag)
		}
		o.nones += none.Len()
		o.values.Write(some)
		return nil
	})
}

func (o *optionBuilder) Build() vector.Any {
	some := o.values.Build()
	if some.Len() == 0 {
		return vector.NewOption(o.typ, vector.NewNone(o.nones))
	}
	if o.nones == 0 {
		return vector.NewOption(o.typ, some)
	}
	return vector.NewOptionBoth(o.typ, o.tags, some, vector.NewNone(o.nones))
}
