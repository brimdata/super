package vector

import (
	"github.com/brimdata/super/scode"
)

type View struct {
	Any
	Index []uint32
}

var _ Any = (*View)(nil)

func NewView(vec Any, index []uint32) *View {
	return &View{vec, index}
}

func (v *View) Len() uint32 {
	return uint32(len(v.Index))
}

func (v *View) Serialize(b *scode.Builder, slot uint32) {
	v.Any.Serialize(b, v.Index[slot])
}
