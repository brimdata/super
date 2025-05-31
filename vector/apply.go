package vector

import "iter"

// Apply applies eval to vecs. If any element of vecs is a Dynamic, Apply rips
// vecs accordingly, applies eval to the ripped vectors, and stitches the
// results together into a Dynamic. If ripUnions is true, Apply also rips
// Unions.
func Apply(ripUnions bool, eval func(...Any) Any, vecs ...Any) Any {
	if ripUnions {
		for k, vec := range vecs {
			if union, ok := Under(vec).(*Union); ok {
				vecs[k] = union.Dynamic
			}
		}
	}
	d, ok := findDynamic(vecs)
	if !ok {
		return eval(vecs...)
	}
	results := make([]Any, len(d.Values))
	for i, ripped := range rip(vecs, d) {
		if len(ripped) > 0 {
			results[i] = Apply(ripUnions, eval, ripped...)
		}
	}
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
		for i, rev := range d.TagMap().Reverse {
			var newVecs []Any
			if len(rev) > 0 {
				for _, vec := range vecs {
					if vec == d {
						newVecs = append(newVecs, d.Values[i])
					} else {
						newVecs = append(newVecs, Pick(vec, rev))
					}
				}
			}
			if !yield(i, newVecs) {
				return
			}
		}
	}
}

// stitch returns a Dynamic for tags and vecs.  If vecs contains any Dynamics,
// stitch flattens them and returns a value containing no nested Dynamics.
func stitch(tags []uint32, vecs []Any) Any {
	var foundDynamic bool
	var newVecsLen int
	for _, vec := range vecs {
		if d, ok := vec.(*Dynamic); ok {
			foundDynamic = true
			newVecsLen += len(d.Values)
		} else {
			newVecsLen++
		}
	}
	if !foundDynamic {
		return NewDynamic(tags, vecs)
	}
	newVecs := make([]Any, 0, newVecsLen)     // vecs but with nested Dynamics replaced by their values
	nestedTags := make([][]uint32, len(vecs)) // tags from nested Dynamics (nil for non-Dynamics)
	shifts := make([]uint32, len(vecs))       // tag + shift[tag] translates tag to newVecs
	var lastShift uint32
	for i, vec := range vecs {
		shifts[i] = lastShift
		if d, ok := vec.(*Dynamic); ok {
			newVecs = append(newVecs, d.Values...)
			nestedTags[i] = d.Tags
			lastShift += uint32(len(d.Values)) - 1
		} else {
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
