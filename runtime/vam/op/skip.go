package op

import (
	"github.com/brimdata/super/vector"
)

type Skip struct {
	parent vector.Puller
	offset int
	count  int
}

func NewSkip(parent vector.Puller, offset int) *Skip {
	return &Skip{
		parent: parent,
		offset: offset,
	}
}

func (o *Skip) Pull(done bool) (vector.Any, error) {
	for {
		vec, err := o.parent.Pull(done)
		if vec == nil || err != nil {
			o.count = 0
			return nil, err
		}
		if o.count >= o.offset {
			return vec, nil
		}
		n := int(vec.Len())
		remaining := o.offset - o.count
		if remaining < n {
			o.count = o.offset
			var offsets []uint32
			for i := remaining; i < n; i++ {
				offsets = append(offsets, uint32(i))
			}
			return vector.Pick(vec, offsets), nil
		}
		o.count += n
	}
}
