package vector

import (
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

// An option value is stored as any vector with its option type.
// The Any field is a None vector for none option values and any
// other vector for some option values.  When somes or nones are
// mixed into a single Option vector, then Any is a two-element
// Dynamic with the first value a some and the second value a none.
// This design relies upon super.OptionSomeTag=0 and super.OptionNoneTag=1.
type Option struct {
	Typ *super.TypeOption
	Any
}

var _ Any = (*Option)(nil)

func NewOption(typ *super.TypeOption, vec Any) *Option {
	return &Option{Typ: typ, Any: vec}
}

func NewOptionBoth(typ *super.TypeOption, tags []uint32, some Any, none *None) Any {
	return NewOption(typ, NewDynamic(tags, []Any{some, none}))
}

// XXX this is called only when there's a mixture of nones and values
func NewOptionFromRLE(sctx *super.Context, vec Any, n uint32, rle []uint32) *Option {
	// XXX we should store RLE natively and generate on demand so that if we
	// write straight to CSUP it would be retained,  maybe just move union RLE here?
	tags, noneLen := buildTags(rle, n)
	optionType := sctx.LookupTypeOption(vec.Type())
	if noneLen == 0 {
		panic("can't be zero")
	}
	return NewOption(optionType, NewDynamic(tags, []Any{vec, NewNone(noneLen)}))
}

func (*Option) Kind() Kind {
	return KindOption
}

func (o *Option) Type() super.Type {
	return o.Typ
}

func (o *Option) Serialize(b *scode.Builder, slot uint32) {
	tag := super.OptionNoneTag
	switch vec := o.Any.(type) {
	case *Dynamic:
		if vec.TypeOf(slot) != super.TypeNone {
			tag = super.OptionSomeTag
		}
	case *None:
	default:
		tag = super.OptionSomeTag
	}
	b.BeginContainer()
	b.Append(super.EncodeUint(uint64(tag)))
	if tag == super.OptionSomeTag {
		o.Any.Serialize(b, slot)
	}
	b.EndContainer()
}

func (o *Option) Apply(f func([]uint32, Any, *None) Any) Any {
	switch vec := o.Any.(type) {
	case *None:
		return f(nil, nil, vec)
	case *Dynamic:
		return f(vec.Tags, vec.Values[0], vec.Values[1].(*None))
	default:
		return f(nil, vec, nil)
	}
}

func DeoptionWithNone(vec Any) Any {
	switch vec := Super(vec).(type) {
	case *None:
		return vec
	case *Dynamic:
		if hasOptionTypesOrNones(vec.Values) {
			vecs := make([]Any, 0, len(vec.Values))
			for _, v := range vec.Values {
				vecs = append(vecs, DeoptionWithNone(v))
			}
			return stitch(vec.Tags, vecs)
		}
	case *Option:
		return DeoptionWithNone(vec.Any)
	}
	return vec
}

func DeoptionWithError(sctx *super.Context, vec, on Any, where string) Any {
	switch vec := vec.(type) {
	case *None:
		return NewWrappedError(sctx, fmt.Sprintf("illegal none value in %s", where), on)
	case *Dynamic:
		if hasOptionTypesOrNones(vec.Values) {
			vecs := make([]Any, 0, len(vec.Values))
			for _, v := range vec.Values {
				vecs = append(vecs, DeoptionWithError(sctx, v, on, where))
			}
			return stitch(vec.Tags, vecs)
		}
	case *Option:
		return DeoptionWithError(sctx, vec.Any, on, where)
	}
	return vec
}

func hasOptionTypesOrNones(vecs []Any) bool {
	return slices.IndexFunc(vecs, func(vec Any) bool {
		// XXX apparently the runtime sometimes creates nil vectors inside
		// of Dynamics where said vector is never referenced by a tag
		// (e.g., at the output of vector switch), so we check for nil here.
		if vec == nil {
			return false
		}
		if vec, ok := vec.(*Dynamic); ok {
			return hasOptionTypesOrNones(vec.Values)
		}
		typ := vec.Type()
		if fusion, ok := typ.(*super.TypeFusion); ok {
			typ = fusion.Type
		}
		return super.IsOptionType(typ) || typ == super.TypeNone
	}) >= 0
}
