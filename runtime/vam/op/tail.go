package op

import (
	"slices"

	"github.com/brimdata/super/vector"
)

type Tail struct {
	parent vector.Puller
	limit  int

	vecs []vector.Any
	eos  bool
}

func NewTail(parent vector.Puller, limit int) *Tail {
	return &Tail{
		parent: parent,
		limit:  limit,
	}
}

func (t *Tail) Pull(done bool) (vector.Any, error) {
	if t.eos {
		// We don't check done here because if we already got EOS,
		// we don't propagate done.
		t.vecs = nil
		t.eos = false
		return nil, nil
	}
	if done {
		t.vecs = nil
		t.eos = false
		return t.parent.Pull(true)
	}
	if len(t.vecs) == 0 {
		vecs, err := t.tail()
		if len(vecs) == 0 || err != nil {
			return nil, err
		}
		t.vecs = vecs
	}
	vec := t.vecs[0]
	t.vecs = t.vecs[1:]
	if len(t.vecs) == 0 {
		t.eos = true
	}
	return vec, nil
}

// tail pulls from t.parent until EOS and returns vectors containing the
// last t.limit values.
func (t *Tail) tail() ([]vector.Any, error) {
	var vecs []vector.Any
	var n int
	for {
		vec, err := t.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if vec == nil {
			break
		}
		vecs = append(vecs, vec)
		n += int(vec.Len())
		for len(vecs) > 0 && n-int(vecs[0].Len()) >= t.limit {
			// We have enough values without vecs[0] so drop it.
			n -= int(vecs[0].Len())
			vecs = slices.Delete(vecs, 0, 1)
		}
	}
	if n > t.limit {
		// We have too many values so remove some from vecs[0].
		extra := n - t.limit
		var index []uint32
		for i := range int(vecs[0].Len()) - extra {
			index = append(index, uint32(extra+i))
		}
		vecs[0] = vector.Pick(vecs[0], index)
	}
	return vecs, nil
}
