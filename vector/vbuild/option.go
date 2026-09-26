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
	switch vec := option.Any.(type) {
	case *vector.None:
		n := vec.Len()
		for range n {
			o.tags = append(o.tags, 0)
		}
		o.nones += n
	case *vector.Dynamic:
		if len(vec.Values) != 2 {
			panic(vec)
		}
		for _, which := range vec.Tags {
			o.tags = append(o.tags, which)
			o.nones++
		}
		o.nones += vec.Values[0].Len()
		o.values.Write(vec.Values[1])
	default:
		o.values.Write(vec)
		for range vec.Len() {
			o.tags = append(o.tags, 1)
		}
	}
}

func (o *optionBuilder) Build() vector.Any {
	vals := o.values.Build()
	if vals.Len() == 0 {
		return vector.NewOption(o.typ, vector.NewNone(o.nones))
	}
	if o.nones == 0 {
		return vector.NewOption(o.typ, vals)
	}
	return vector.NewOption(o.typ, vector.NewDynamic(o.tags, []vector.Any{vector.NewNone(o.nones), vals}))
}
