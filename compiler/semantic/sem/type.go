package sem

import "github.com/brimdata/super"

type Type struct {
	typ super.Type
}

func (t *Type) SetType(typ super.Type) {
	t.typ = typ
}

func (t *Type) GetType() super.Type {
	return t.typ
}
