package commits

import (
	"errors"
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/lake/data"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/runtime/sam/expr/extent"
	"github.com/brimdata/zed/zngbytes"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
)

var ErrWriteConflict = errors.New("write conflict")

type View interface {
	Lookup(ksuid.KSUID) (*data.Object, error)
	HasVector(ksuid.KSUID) bool
	Select(extent.Span, order.Which) DataObjects
	SelectAll() DataObjects
}

type Writeable interface {
	View
	AddDataObject(*data.Object) error
	DeleteObject(ksuid.KSUID) error
	AddVector(ksuid.KSUID) error
	DeleteVector(ksuid.KSUID) error
}

// A snapshot summarizes the pool state at any point in
// the commit object tree.
// XXX redefine snapshot as type map instead of struct
type Snapshot struct {
	objects map[ksuid.KSUID]*data.Object
	vectors map[ksuid.KSUID]struct{}
}

var _ View = (*Snapshot)(nil)
var _ Writeable = (*Snapshot)(nil)

func NewSnapshot() *Snapshot {
	return &Snapshot{
		objects: make(map[ksuid.KSUID]*data.Object),
		vectors: make(map[ksuid.KSUID]struct{}),
	}
}

func (s *Snapshot) AddDataObject(object *data.Object) error {
	id := object.ID
	if _, ok := s.objects[id]; ok {
		return fmt.Errorf("%s: add of a duplicate data object: %w", id, ErrWriteConflict)
	}
	s.objects[id] = object
	return nil
}

func (s *Snapshot) DeleteObject(id ksuid.KSUID) error {
	if _, ok := s.objects[id]; !ok {
		return fmt.Errorf("%s: delete of a non-existent data object: %w", id, ErrWriteConflict)
	}
	delete(s.objects, id)
	return nil
}

func (s *Snapshot) AddVector(id ksuid.KSUID) error {
	if _, ok := s.vectors[id]; ok {
		return fmt.Errorf("%s: add of a duplicate vector of data object: %w", id, ErrWriteConflict)
	}
	s.vectors[id] = struct{}{}
	return nil
}

func (s *Snapshot) DeleteVector(id ksuid.KSUID) error {
	if _, ok := s.vectors[id]; !ok {
		return fmt.Errorf("%s: delete of a non-present vector: %w", id, ErrWriteConflict)
	}
	delete(s.vectors, id)
	return nil
}

func Exists(view View, id ksuid.KSUID) bool {
	_, err := view.Lookup(id)
	return err == nil
}

func (s *Snapshot) Exists(id ksuid.KSUID) bool {
	return Exists(s, id)
}

func (s *Snapshot) Lookup(id ksuid.KSUID) (*data.Object, error) {
	o, ok := s.objects[id]
	if !ok {
		return nil, fmt.Errorf("%s: %w", id, ErrNotFound)
	}
	return o, nil
}

func (s *Snapshot) HasVector(id ksuid.KSUID) bool {
	_, ok := s.vectors[id]
	return ok
}

func (s *Snapshot) Select(scan extent.Span, order order.Which) DataObjects {
	var objects DataObjects
	for _, o := range s.objects {
		segspan := o.Span(order)
		if scan == nil || segspan == nil || extent.Overlaps(scan, segspan) {
			objects = append(objects, o)
		}
	}
	return objects
}

func (s *Snapshot) SelectAll() DataObjects {
	var objects DataObjects
	for _, o := range s.objects {
		objects = append(objects, o)
	}
	return objects
}

func (s *Snapshot) Copy() *Snapshot {
	out := NewSnapshot()
	for key, val := range s.objects {
		out.objects[key] = val
	}
	for key := range s.vectors {
		out.vectors[key] = struct{}{}
	}
	return out
}

// serialize serializes a snapshot as a sequence of actions.  Commit IDs are
// omitted from actions since they are neither available here nor required
// during deserialization.  Deleted entities are serialized as an add-delete
// sequence to meet the requirements of DeleteObject.
func (s *Snapshot) serialize() ([]byte, error) {
	zs := zngbytes.NewSerializer()
	zs.Decorate(zson.StylePackage)
	for _, o := range s.objects {
		if err := zs.Write(&Add{Object: *o}); err != nil {
			return nil, err
		}
	}
	for id := range s.vectors {
		if err := zs.Write(&AddVector{ID: id}); err != nil {
			return nil, err
		}
	}
	if err := zs.Close(); err != nil {
		return nil, err
	}
	return zs.Bytes(), nil
}

func decodeSnapshot(zctx *zed.Context, r io.Reader) (*Snapshot, error) {
	arena := zed.NewArena()
	s := NewSnapshot()
	zd := zngbytes.NewDeserializer(zctx, arena, r, ActionTypes)
	defer zd.Close()
	for {
		entry, err := zd.Read()
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return s, nil
		}
		action, ok := entry.(Action)
		if !ok {
			return nil, fmt.Errorf("internal error: corrupt snapshot contains unknown entry type %T", entry)
		}
		setActionArena(action, arena)
		if err := PlayAction(s, action); err != nil {
			return nil, err
		}
	}
}

type DataObjects []*data.Object

func (d *DataObjects) Append(objects DataObjects) {
	*d = append(*d, objects...)
}

func PlayAction(w Writeable, action Action) error {
	switch action := action.(type) {
	case *Add:
		return w.AddDataObject(&action.Object)
	case *Delete:
		return w.DeleteObject(action.ID)
	case *AddVector:
		return w.AddVector(action.ID)
	case *DeleteVector:
		return w.DeleteVector(action.ID)
	case *Commit:
		// ignore
		return nil
	}
	return fmt.Errorf("lake.commits.PlayAction: unknown action %T", action)
}

// Play "plays" a recorded transaction into a writeable snapshot.
func Play(w Writeable, o *Object) error {
	for _, a := range o.Actions {
		if err := PlayAction(w, a); err != nil {
			return err
		}
	}
	return nil
}

func Vectors(view View) *Snapshot {
	snap := NewSnapshot()
	for _, o := range view.SelectAll() {
		if view.HasVector(o.ID) {
			snap.AddDataObject(o)
		}
	}
	return snap
}
