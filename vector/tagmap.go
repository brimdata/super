package vector

// TagMap is used by dynamics and unions to map slots between parent and child in
// both the forward and reverse directions. We need this because vectors are stored
// in a dense format where different types hold only the values needed for that type.
// If we stored vectors in a sparse format, the amount of overhead would increase
// substantially for heterogeneously typed data.
type TagMap struct {
	Forward []uint32
	Reverse [][]uint32
}

func NewTagMap(tags []uint32, vals []Any) *TagMap {
	lens := make([]uint32, 0, len(vals))
	for _, v := range vals {
		var length uint32
		if v != nil {
			length = v.Len()
		}
		lens = append(lens, length)
	}
	return NewTagMapFromLens(tags, lens)
}

func NewTagMapFromLens(tags []uint32, lens []uint32) *TagMap {
	forward := make([]uint32, len(tags))
	space := make([]uint32, len(tags))
	reverse := make([][]uint32, len(lens))
	var off uint32
	for tag, n := range lens {
		reverse[tag] = space[off : off+n]
		off += n
	}
	if off != uint32(len(tags)) {
		//XXX this can happen for corrupt tags arrays... need to sanity
		// check them when we load.
		//XXX make this more reasonable (check when tags are read in vcache)
		panic("bad VNG tagmap")
	}
	counts := make([]uint32, len(lens))
	for slot, tag := range tags {
		childSlot := counts[tag]
		counts[tag]++
		forward[slot] = childSlot
		reverse[tag][childSlot] = uint32(slot)
	}
	return &TagMap{
		Forward: forward,
		Reverse: reverse,
	}
}
