package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

//XXX update comment

// A None vector arises from values not present in an optional field.
// In a future version of the runtime, we will have operators
// that handle noneness (?? and ?.) but for now the only
// thing you can do with none is assign it to a optional
// record field or express it as missing.  None wraps Error as
// an error("missing") so it expresses this when not assigned to
// a field.
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
// XXX fix this to make a vector.Option
func NewNoneOption(sctx *super.Context, typ super.Type, length uint32) *Union {
	union, noneTag := super.OptionUnion(typ)
	if union == nil {
		panic(sup.FormatType(typ))
	}
	tags := make([]uint32, length)
	for k := range length {
		tags[k] = uint32(noneTag)
	}
	var valTag int
	if noneTag == 0 {
		valTag = 1
	}
	vecs := make([]Any, 2)
	vecs[noneTag] = NewNone(length)
	vecs[valTag] = NewEmpty(typ)
	return NewUnion(union, tags, vecs)
}
