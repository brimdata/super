package vector

import (
	"iter"
)

type ApplyOpt uint

const (
	ApplyNone      ApplyOpt = 0
	ApplyRipUnions ApplyOpt = 1 << iota
	ApplyRipFusions
)

type NoRip struct {
	Any
}

// Apply applies eval to vecs. If any element of vecs is a Dynamic, Apply rips
// vecs accordingly, applies eval to the ripped vectors, and stitches the
// results together into a Dynamic.
func Apply(opt ApplyOpt, eval func(...Any) Any, vecs ...Any) Any {
	if opt&ApplyRipFusions != 0 {
		for k, vec := range vecs {
			vecs[k] = Super(vec)
		}
	}
	if opt&ApplyRipUnions != 0 {
		for k, vec := range vecs {
			if vec, ok := Under(vec).(*Union); ok {
				vecs[k] = vec.Dynamic()
			}
		}
	}
	d, ok := findDynamic(vecs)
	if !ok {
		return eval(vecs...)
	}
	results := make([]Any, len(d.Values))
	for i, ripped := range rip(vecs, d) {
		results[i] = Apply(opt, eval, ripped...)
	}
	// stitch removes nils and replaces Dynamics with their values.
	return stitch(d.Tags, results)
}

func findDynamic(vecs []Any) (*Dynamic, bool) {
	for _, vec := range vecs {
		if d, ok := vec.(*Dynamic); ok {
			return d, true
		}
	}
	return nil, false
}

func rip(vecs []Any, d *Dynamic) iter.Seq2[int, []Any] {
	return func(yield func(int, []Any) bool) {
		for i, rev := range d.ReverseTagMap() {
			if len(rev) == 0 {
				continue
			}
			ripped := make([]Any, len(vecs))
			for j, vec := range vecs {
				if vec == d {
					ripped[j] = d.Values[i]
				} else {
					ripped[j] = Pick(vec, rev)
				}
			}
			if !yield(i, ripped) {
				return
			}
		}
	}
}

// stitch returns a Dynamic for tags and vecs with nil entries removed and
// Dynamic entries replaced by their values (i.e., it flattens one level of
// Dynamic).
func stitch(tags []uint32, vecs []Any) Any {
	var needStitch bool
	var newVecsLen int
	for _, vec := range vecs {
		switch vec := vec.(type) {
		case nil:
			needStitch = true
		case *Dynamic:
			needStitch = true
			newVecsLen += len(vec.Values)
		default:
			newVecsLen++
		}
	}
	if !needStitch {
		return NewDynamic(tags, vecs)
	}
	newVecs := make([]Any, 0, newVecsLen)     // vecs but without nils and with Dynamics replaced by their values
	nestedTags := make([][]uint32, len(vecs)) // tags from nested Dynamics (nil for non-Dynamics)
	shifts := make([]uint32, len(vecs))       // tag + shift[tag] translates tag to newVecs
	var lastShift uint32
	for i, vec := range vecs {
		shifts[i] = lastShift
		switch vec := vec.(type) {
		case nil:
			lastShift--
		case *Dynamic:
			newVecs = append(newVecs, vec.Values...)
			nestedTags[i] = vec.Tags
			lastShift += uint32(len(vec.Values)) - 1
		default:
			newVecs = append(newVecs, vec)
		}
	}
	newTags := make([]uint32, len(tags))
	for i, t := range tags {
		newTag := t + shifts[t]
		if nested := nestedTags[t]; len(nested) > 0 {
			newTag += nested[0]
			nestedTags[t] = nested[1:]
		}
		newTags[i] = newTag
	}
	return NewDynamic(newTags, newVecs)
}

func AddNoRip(vec Any) Any {
	if dynamic, ok := vec.(*Dynamic); ok {
		vals := make([]Any, len(dynamic.Values))
		for i, vec := range dynamic.Values {
			vals[i] = AddNoRip(vec)
		}
		return NewDynamic(dynamic.Tags, vals)
	}
	return &NoRip{vec}
}

func ClearNoRips(vecs []Any) {
	for i, vec := range vecs {
		if norip, ok := vec.(*NoRip); ok {
			vecs[i] = norip.Any
		}
	}
}
