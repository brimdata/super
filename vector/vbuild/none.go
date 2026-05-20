package vbuild

import (
	"github.com/brimdata/super/vector"
)

type noneBuilder struct {
	len uint32
}

func (n *noneBuilder) Write(vec vector.Any) {
	n.len += vec.(*vector.None).Len()
}

func (n *noneBuilder) Build() vector.Any {
	return vector.NewNone(n.len)
}

type nullBuilder struct {
	len uint32
}

func (n *nullBuilder) Write(vec vector.Any) {
	n.len += vec.(*vector.Null).Len()
}

func (n *nullBuilder) Build() vector.Any {
	return vector.NewNull(n.len)
}
