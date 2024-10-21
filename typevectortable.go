package super

import "slices"

type TypeVectorTable struct {
	types []typeVector
}

func NewTypeVectorTable() *TypeVectorTable {
	return &TypeVectorTable{}
}

func (t *TypeVectorTable) Lookup(types []Type) int {
	for k, typ := range t.types {
		if typ.equal(types) {
			return k
		}
	}
	k := len(t.types)
	t.types = append(t.types, newTypeVector(types))
	return k
}

func (t *TypeVectorTable) LookupByValues(vals []Value) int {
	for k, typ := range t.types {
		if typ.equalToValues(vals) {
			return k
		}
	}
	k := len(t.types)
	t.types = append(t.types, newTypeVectorFromValues(vals))
	return k
}

func (t *TypeVectorTable) Types(id int) []Type {
	return t.types[id]
}

func (t *TypeVectorTable) Length() int {
	return len(t.types)
}

type typeVector []Type

func newTypeVector(in []Type) typeVector {
	return slices.Clone(in)
}

func newTypeVectorFromValues(vals []Value) typeVector {
	out := make(typeVector, 0, len(vals))
	for _, val := range vals {
		out = append(out, val.Type())
	}
	return out
}

func (t typeVector) equal(to []Type) bool {
	return slices.Equal(t, to)
}

func (t typeVector) equalToValues(vals []Value) bool {
	return slices.EqualFunc(t, vals, func(typ Type, val Value) bool {
		return typ == val.Type()
	})
}
