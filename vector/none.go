package vector

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

// A None vector arises from values not present in option types.
// In general, nones outside of options types should be handled by the query
// and ultimately turned into errors and never serialized.  However, it
// is possible to serialize them and inspect them when necessary for debugging etc.
type None struct {
	len uint32
}

func NewNone(len uint32) *None {
	return &None{len}
}

func (*None) Kind() Kind {
	return KindNone
}

func (n *None) Len() uint32 {
	return n.len
}

func (*None) Serialize(b *scode.Builder, _ uint32) {
	b.Append(nil)
}

func (*None) Type() super.Type {
	return super.TypeNone
}

//	Make an option type as a union and all of the none type.
//
// XXX a subsequent PR will change this logic to make a vector.Option when
// there is a non-union some type.
func NewOptionNone(sctx *super.Context, optionType *super.TypeUnion, length uint32) *Union {
	u, noneTag := super.OptionUnion(optionType)
	if u == nil {
		panic(sup.FormatType(optionType))
	}
	tags := make([]uint32, length)
	for k := range length {
		tags[k] = uint32(noneTag)
	}
	vecs := make([]Any, len(optionType.Types))
	vecs[noneTag] = NewNone(length)
	for tag, typ := range optionType.Types {
		if tag != noneTag {
			vecs[tag] = NewEmpty(typ)
		}
	}
	return NewUnion(optionType, tags, vecs)
}

func NewOptionSome(sctx *super.Context, vec Any) Any {
	typ := vec.Type()
	if super.IsOptionType(typ) {
		return vec
	}
	optionType := sctx.Option(typ)
	if union, ok := vec.(*Union); ok {
		vecs := slices.Clone(union.Values())
		vecs = append(vecs, NewNone(0))
		//XXX We should copy RLEs instead of making tags.
		// We will fix this in a subsequent PR.
		return NewUnion(optionType, union.Tags(), vecs)
	}
	return NewUnion(optionType, make([]uint32, vec.Len()), []Any{vec, NewNone(0)})
}
