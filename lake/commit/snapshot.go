package commit

import (
	"errors"

	"github.com/brimdata/zed/expr/extent"
	"github.com/brimdata/zed/lake/commit/actions"
	"github.com/brimdata/zed/lake/segment"
	"github.com/brimdata/zed/order"
	"github.com/segmentio/ksuid"
)

var (
	ErrExists   = errors.New("segment exists")
	ErrNotFound = errors.New("segment not found")
)

type View interface {
	Lookup(ksuid.KSUID) (*segment.Reference, error)
	Select(extent.Span, order.Which) Segments
	SelectAll() Segments
}

type Writeable interface {
	View
	AddSegment(seg *segment.Reference) error
	DeleteSegment(id ksuid.KSUID) error
}

// A snapshot summarizes the pool state at a given point in the journal.
type Snapshot struct {
	segments map[ksuid.KSUID]*segment.Reference
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		segments: make(map[ksuid.KSUID]*segment.Reference),
	}
}

func (s *Snapshot) AddSegment(seg *segment.Reference) error {
	id := seg.ID
	if _, ok := s.segments[id]; ok {
		return ErrExists
	}
	s.segments[id] = seg
	return nil
}

func (s *Snapshot) DeleteSegment(id ksuid.KSUID) error {
	if _, ok := s.segments[id]; !ok {
		return ErrNotFound
	}
	delete(s.segments, id)
	return nil
}

func Exists(view View, id ksuid.KSUID) bool {
	_, err := view.Lookup(id)
	return err == nil
}

func (s *Snapshot) Exists(id ksuid.KSUID) bool {
	return Exists(s, id)
}

func (s *Snapshot) Lookup(id ksuid.KSUID) (*segment.Reference, error) {
	seg, ok := s.segments[id]
	if !ok {
		return nil, ErrNotFound
	}
	return seg, nil
}

func (s *Snapshot) Select(scan extent.Span, o order.Which) Segments {
	var segments Segments
	for _, seg := range s.segments {
		segspan := seg.Span(o)
		if scan == nil || segspan == nil || extent.Overlaps(scan, segspan) {
			segments = append(segments, seg)
		}
	}
	return segments
}

func (s *Snapshot) SelectAll() Segments {
	var segments Segments
	for _, seg := range s.segments {
		segments = append(segments, seg)
	}
	return segments
}

type Segments []*segment.Reference

func (s *Segments) Append(segments Segments) {
	*s = append(*s, segments...)
}

func PlayAction(w Writeable, action actions.Interface) error {
	//XXX other cases like actions.AddIndex etc coming soon...
	switch action := action.(type) {
	case *actions.Add:
		w.AddSegment(&action.Segment)
	case *actions.Delete:
		w.DeleteSegment(action.ID)
	}
	return nil
}

// Play "plays" a recorded transaction into a writeable snapshot.
func Play(w Writeable, txn *Transaction) error {
	for _, a := range txn.Actions {
		if err := PlayAction(w, a); err != nil {
			return err
		}
	}
	return nil
}
