package demand

import (
	"maps"

	"github.com/brimdata/super/pkg/field"
)

type Demand interface {
	isDemand()
}

func (demand all) isDemand()  {}
func (demand keys) isDemand() {}

type all struct{}
type keys map[string]Demand // No empty values.

func IsValid(demand Demand) bool {
	switch demand := demand.(type) {
	case nil:
		return false
	case all:
		return true
	case keys:
		for _, v := range demand {
			if !IsValid(v) || IsNone(v) {
				return false
			}
		}
		return true
	default:
		panic("Unreachable")
	}
}

func None() Demand {
	return keys{}
}

func All() Demand {
	return all{}
}

func IsNone(demand Demand) bool {
	switch demand := demand.(type) {
	case all:
		return false
	case keys:
		return len(demand) == 0
	default:
		panic("Unreachable")
	}
}

func IsAll(demand Demand) bool {
	_, ok := demand.(all)
	return ok
}

func Key(key string, value Demand) Demand {
	if IsNone(value) {
		return value
	}
	return keys{key: value}
}

// Delete deletes entries in b from a.
func Delete(a, b Demand) Demand {
	aa, ok := a.(keys)
	if !ok {
		return a
	}
	bb, ok := b.(keys)
	if !ok {
		return a
	}
	copyOnWrite := true
	for k, bv := range bb {
		av, ok := aa[k]
		if !ok {
			continue
		}
		if copyOnWrite {
			aa = maps.Clone(aa)
			copyOnWrite = false
		}
		if IsAll(bv) {
			delete(aa, k)
			continue
		}
		aa[k] = Delete(av, bv)
	}
	return aa
}

func Union(demands ...Demand) Demand {
	out := None().(keys)
	for _, d := range demands {
		switch d := d.(type) {
		case all:
			return All()
		case keys:
			for k, v := range d {
				if v2, ok := out[k]; ok {
					out[k] = Union(v, v2)
				} else {
					out[k] = v
				}
			}
		default:
			panic("Unreachable")
		}
	}
	return out
}

func GetKey(demand Demand, key string) Demand {
	switch demand := demand.(type) {
	case all:
		return demand
	case keys:
		if value, ok := demand[key]; ok {
			return value
		}
		return None()
	default:
		panic("Unreachable")
	}
}

func Fields(d Demand) []field.Path {
	keys, ok := d.(keys)
	if !ok {
		return nil
	}
	var fields []field.Path
	for k, v := range keys {
		if fs := Fields(v); len(fs) > 0 {
			for _, f := range fs {
				fields = append(fields, append(field.Path{k}, f...))
			}
		} else {
			fields = append(fields, field.Path{k})
		}
	}
	return fields
}
