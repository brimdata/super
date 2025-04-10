package op

import (
	"github.com/brimdata/super/vector"
)

type Head struct {
	parent       vector.Puller
	limit, count int
}

func NewHead(parent vector.Puller, limit int) *Head {
	return &Head{
		parent: parent,
		limit:  limit,
	}
}

func (h *Head) Pull(done bool) (vector.Any, error) {
	if h.count >= h.limit {
		// If we are at limit we already sent a done upstream,
		// so for either sense of the done flag, we return EOS
		// and reset our state.
		h.count = 0
		return nil, nil
	}
	if done {
		if _, err := h.parent.Pull(true); err != nil {
			return nil, err
		}
		h.count = 0
		return nil, nil
	}
again:
	vec, err := h.parent.Pull(false)
	if vec == nil || err != nil {
		h.count = 0
		return nil, err
	}
	remaining := h.limit - h.count
	if remaining <= 0 {
		goto again
	}
	if n := int(vec.Len()); n < remaining {
		// This vector has fewer than the needed records.
		// Send them all downstream and update the count.
		h.count += n
		return vec, nil
	}
	// This vector has more than the needed records.
	// Signal to the parent that we are done.
	if _, err := h.parent.Pull(true); err != nil {
		return nil, err
	}
	h.count = h.limit
	index := make([]uint32, remaining)
	for k := range index {
		index[k] = uint32(k)
	}
	return vector.Pick(vec, index), nil
}
