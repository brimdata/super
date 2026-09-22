package vector

import (
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Option struct {
	Typ *super.TypeOption
	Any
}

var _ Any = (*Option)(nil)

// XXX vec of type T because a Some(T), a None becomes Option(None),
// or a Dynamic to have a mixture
func NewOption(typ *super.TypeOption, vec Any) *Option {
	return &Option{Typ: typ, Any: vec}
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
	// Reverse the tags since fjson presumes none is last.
	// XXX maybe we should change Option type to have none last instead of first in Dynamic
	for k, tag := range tags {
		if tag == 0 {
			tags[k] = 1
		} else {
			tags[k] = 0
		}
	}
	//fmt.Println("OPTION FROM RLE", sup.String(optionType), Format(vec))
	//fmt.Println("NONE LEN", noneLen, "TAGS", tags)
	return NewOption(optionType, NewDynamic(tags, []Any{NewNone(noneLen), vec}))
}

func (*Option) Kind() Kind {
	return KindOption
}

func (o *Option) Type() super.Type {
	return o.Typ
}

func (o *Option) Serialize(b *scode.Builder, slot uint32) {
	var which uint64
	switch vec := o.Any.(type) {
	case *Dynamic:
		if vec.TypeOf(slot) != super.TypeNone {
			which = 1
		}
	case *None:
	default:
		which = 1
	}
	b.BeginContainer()
	b.Append(super.EncodeUint(which))
	if which != 0 {
		o.Any.Serialize(b, slot)
	}
	b.EndContainer()
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

//	Make an option type as a union and all of the none type.
//
// XXX a subsequent PR will change this logic to make a vector.Option when
// there is a non-union some type.
func NewOptionNone(optionType *super.TypeOption, length uint32) *Option {
	return NewOption(optionType, NewNone(length))
}

func NewOptionSome(sctx *super.Context, vec Any) Any {
	typ := vec.Type()
	if super.IsOptionType(typ) {
		return vec
	}
	//XXX check for vec is None?
	return NewOption(sctx.LookupTypeOption(typ), vec)
}
