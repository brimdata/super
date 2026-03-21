package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type TypeValue struct {
	sctx   *super.Context
	Values []super.Type
}

var _ Any = (*TypeValue)(nil)

func NewTypeValue(sctx *super.Context, vals []super.Type) *TypeValue {
	return &TypeValue{Values: vals}
}

func (t *TypeValue) Append(typ super.Type) {
	t.Values = append(t.Values, typ)
}

func (*TypeValue) Kind() Kind {
	return KindType
}

func (t *TypeValue) Type() super.Type {
	return super.TypeType
}

func (t *TypeValue) Len() uint32 {
	return uint32(len(t.Values))
}

func (t *TypeValue) Value(slot uint32) super.Type {
	return t.Values[slot]
}

func (t *TypeValue) Serialize(b *scode.Builder, slot uint32) {
	b.Append(t.sctx.LookupTypeValue(t.Values[slot]).Bytes())
}

// XXX temporary until we can switch over to typedef table in CSUP
func (t *TypeValue) Table() BytesTable {
	table := NewBytesTableEmpty(t.Len())
	for slot := range t.Len() {
		table.Append(t.sctx.LookupTypeValue(t.Values[slot]).Bytes())
	}
	return table
}

/* XXX
func TypeValueValue(val Any, slot uint32) []byte {
	switch val := val.(type) {
	case *TypeValue:
		return val.Value(slot)
	case *Const:
		return TypeValueValue(val.Any, 0)
	case *Dict:
		slot = uint32(val.Index[slot])
		return val.Any.(*TypeValue).Value(slot)
	case *View:
		slot = val.Index[slot]
		return TypeValueValue(val.Any, slot)
	}
	panic(val)
}
*/
