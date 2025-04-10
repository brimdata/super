package vector

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
	var results []Any
	for _, ripped := range rip(vecs, d) {
		var result Any
		if len(ripped) > 0 {
			result = Apply(ripUnions, eval, ripped...)
		}
		results = append(results, result)
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

func rip(vecs []Any, d *Dynamic) [][]Any {
	var ripped [][]Any
	for j, rev := range d.TagMap.Reverse {
		var newVecs []Any
		if len(rev) > 0 {
			for _, vec := range vecs {
				if vec == d {
					newVecs = append(newVecs, d.Values[j])
				} else {
					newVecs = append(newVecs, Pick(vec, rev))
				}
			}
		}
		ripped = append(ripped, newVecs)
	}
	return ripped
}

// stitch returns a Dynamic for tags and vecs.  If vecs contains any Dynamics,
// stitch flattens them and returns a value containing no nested Dynamics.
func stitch(tags []uint32, vecs []Any) Any {
	if _, ok := findDynamic(vecs); !ok {
		return NewDynamic(tags, vecs)
	}
	var nestedTags [][]uint32 // tags from nested Dynamics (nil for non-Dynamics)
	var newVecs []Any         // vecs but with nested Dynamics replaced by their values
	var shifts []uint32       // tag + shift[tag] translates tag to newVecs
	var lastShift uint32
	for _, vec := range vecs {
		shifts = append(shifts, lastShift)
		if d, ok := vec.(*Dynamic); ok {
			nestedTags = append(nestedTags, d.Tags)
			newVecs = append(newVecs, d.Values...)
			lastShift += uint32(len(d.Values)) - 1
		} else {
			nestedTags = append(nestedTags, nil)
			newVecs = append(newVecs, vec)
		}
	}
	var newTags []uint32
	for _, t := range tags {
		newTag := t + shifts[t]
		if nested := nestedTags[t]; len(nested) > 0 {
			newTag += nested[0]
			nestedTags[t] = nested[1:]
		}
		newTags = append(newTags, newTag)
	}
	return NewDynamic(newTags, newVecs)
}
