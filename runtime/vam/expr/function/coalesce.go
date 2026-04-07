package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Coalesce struct{}

func (*Coalesce) RipUnions() bool { return false }

func (c *Coalesce) Call(vecs ...vector.Any) vector.Any {
	n := vecs[0].Len()
	// pending tracks slots that still need a value; nil means all slots.
	var pending []uint32

	type part struct {
		indices []uint32
		vec     vector.Any
	}
	var parts []part

	for _, vec := range vecs {
		k := vec.Kind()
		if k == vector.KindNull || k == vector.KindError {
			continue
		}
		nullSlots := nullableSlots(vec, pending, n)
		if nullSlots == nil {
			// vec is non-null for all relevant slots
			if pending == nil {
				return vec
			}
			parts = append(parts, part{pending, vector.Pick(vec, pending)})
			pending = nil
			break
		}
		// vec is a nullable union with some null slots; split pending
		slots := pending
		if slots == nil {
			slots = makeSlots(n)
		}
		nonNull := exclude(slots, nullSlots)
		if len(nonNull) > 0 {
			parts = append(parts, part{nonNull, vector.Pick(vec, nonNull)})
		}
		pending = nullSlots
		if len(pending) == 0 {
			break
		}
	}

	if len(parts) == 0 {
		return vector.NewNull(n)
	}

	// Build a Dynamic where each slot comes from the first non-null part.
	tags := make([]uint32, n)
	vecs2 := make([]vector.Any, len(parts)+1)
	for i, p := range parts {
		vecs2[i+1] = p.vec
		for _, slot := range p.indices {
			tags[slot] = uint32(i + 1)
		}
	}
	var nullCount uint32
	for _, t := range tags {
		if t == 0 {
			nullCount++
		}
	}
	vecs2[0] = vector.NewNull(nullCount)
	return vector.NewDynamic(tags, vecs2)
}

// nullableSlots returns the slots in pending (all slots when pending is nil)
// where vec is a nullable union and the slot value is null. Returns nil if
// vec is not a nullable union or has no null slots among the relevant slots.
func nullableSlots(vec vector.Any, pending []uint32, n uint32) []uint32 {
	u, ok := vec.(*vector.Union)
	if !ok {
		return nil
	}
	nullTag := u.Typ.TagOf(super.TypeNull)
	if nullTag < 0 {
		return nil
	}
	slots := pending
	if slots == nil {
		slots = makeSlots(n)
	}
	var result []uint32
	for _, slot := range slots {
		if int(u.Tags[slot]) == nullTag {
			result = append(result, slot)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// makeSlots returns a slice [0, 1, ..., n-1].
func makeSlots(n uint32) []uint32 {
	s := make([]uint32, n)
	for i := range s {
		s[i] = uint32(i)
	}
	return s
}

// exclude returns slots with all elements of drop removed.
// drop must be a sorted subset of slots.
func exclude(slots, drop []uint32) []uint32 {
	if len(drop) == 0 {
		return slots
	}
	result := make([]uint32, 0, len(slots)-len(drop))
	di := 0
	for _, s := range slots {
		if di < len(drop) && s == drop[di] {
			di++
		} else {
			result = append(result, s)
		}
	}
	return result
}
