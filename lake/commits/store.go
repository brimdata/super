package commits

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/bsupbytes"
	"github.com/brimdata/super/lake/data"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/bsupio"
	arc "github.com/hashicorp/golang-lru/arc/v2"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
)

var (
	ErrBadCommitObject = errors.New("first record of object not a commit")
	ErrExists          = errors.New("commit object already exists")
	ErrNotFound        = errors.New("commit object not found")
)

type Store struct {
	engine storage.Engine
	logger *zap.Logger
	path   *storage.URI

	cache     *arc.ARCCache[ksuid.KSUID, *Object]
	paths     *arc.ARCCache[ksuid.KSUID, []ksuid.KSUID]
	snapshots *arc.ARCCache[ksuid.KSUID, *Snapshot]
}

func OpenStore(engine storage.Engine, logger *zap.Logger, path *storage.URI) (*Store, error) {
	cache, err := arc.NewARC[ksuid.KSUID, *Object](1024)
	if err != nil {
		return nil, err
	}
	paths, err := arc.NewARC[ksuid.KSUID, []ksuid.KSUID](1024)
	if err != nil {
		return nil, err
	}
	snapshots, err := arc.NewARC[ksuid.KSUID, *Snapshot](32)
	if err != nil {
		return nil, err
	}
	return &Store{
		engine:    engine,
		logger:    logger.Named("commits"),
		path:      path,
		cache:     cache,
		paths:     paths,
		snapshots: snapshots,
	}, nil
}

func (s *Store) Get(ctx context.Context, commit ksuid.KSUID) (*Object, error) {
	if o, ok := s.cache.Get(commit); ok {
		return o, nil
	}
	r, err := s.engine.Get(ctx, s.pathOf(commit))
	if err != nil {
		return nil, err
	}
	o, err := DecodeObject(r)
	if err == ErrBadCommitObject {
		err = fmt.Errorf("system error: %s: %w", s.pathOf(commit), ErrBadCommitObject)
	}
	if closeErr := r.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	s.cache.Add(commit, o)
	return o, nil
}

func (s *Store) pathOf(commit ksuid.KSUID) *storage.URI {
	return s.path.JoinPath(commit.String() + ".bsup")
}

func (s *Store) Put(ctx context.Context, o *Object) error {
	b, err := o.Serialize()
	if err != nil {
		return err
	}
	return storage.Put(ctx, s.engine, s.pathOf(o.Commit), bytes.NewReader(b))
}

// DANGER ZONE - objects should only be removed when GC says they are not used.
func (s *Store) Remove(ctx context.Context, o *Object) error {
	return s.engine.Delete(ctx, s.pathOf(o.Commit))
}

func (s *Store) Snapshot(ctx context.Context, leaf ksuid.KSUID) (*Snapshot, error) {
	if snap, ok := s.snapshots.Get(leaf); ok {
		return snap, nil
	}
	if snap, err := s.getSnapshot(ctx, leaf); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.logger.Error("Loading snapshot", zap.Error(err))
	} else if err == nil {
		s.snapshots.Add(leaf, snap)
		return snap, nil
	}
	var objects []*Object
	var base *Snapshot
	for at := leaf; at != ksuid.Nil; {
		if snap, ok := s.snapshots.Get(at); ok {
			base = snap
			break
		}
		var o *Object
		var oErr error
		var wg sync.WaitGroup
		wg.Add(1)
		// Start fetching the next data object.
		go func() {
			o, oErr = s.Get(ctx, at)
			wg.Done()
		}()
		// Concurrently check for a snapshot.
		if snap, err := s.getSnapshot(ctx, at); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.logger.Error("Loading snapshot", zap.Error(err))
		} else if err == nil {
			s.snapshots.Add(at, snap)
			base = snap
			break
		}
		// No snapshot found, so wait for data object.
		wg.Wait()
		if oErr != nil {
			return nil, oErr
		}
		objects = append(objects, o)
		at = o.Parent
	}
	var snap *Snapshot
	if base == nil {
		snap = NewSnapshot()
	} else {
		snap = base.Copy()
	}
	for k := len(objects) - 1; k >= 0; k-- {
		for _, action := range objects[k].Actions {
			if err := PlayAction(snap, action); err != nil {
				return nil, err
			}
		}
	}
	if err := s.putSnapshot(ctx, leaf, snap); err != nil {
		s.logger.Error("Storing snapshot", zap.Error(err))
	}
	s.snapshots.Add(leaf, snap)
	return snap, nil
}

func (s *Store) getSnapshot(ctx context.Context, commit ksuid.KSUID) (*Snapshot, error) {
	r, err := s.engine.Get(ctx, s.snapshotPathOf(commit))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return decodeSnapshot(r)
}

func (s *Store) putSnapshot(ctx context.Context, commit ksuid.KSUID, snap *Snapshot) error {
	b, err := snap.serialize()
	if err != nil {
		return err
	}
	return storage.Put(ctx, s.engine, s.snapshotPathOf(commit), bytes.NewReader(b))
}

func (s *Store) snapshotPathOf(commit ksuid.KSUID) *storage.URI {
	return s.path.JoinPath(commit.String() + ".snap.bsup")
}

// Path return the entire path from the commit object to the root
// in leaf to root order.
func (s *Store) Path(ctx context.Context, leaf ksuid.KSUID) ([]ksuid.KSUID, error) {
	if leaf == ksuid.Nil {
		return nil, errors.New("no path for nil commit ID")
	}
	if path, ok := s.paths.Get(leaf); ok {
		return path, nil
	}
	path, err := s.PathRange(ctx, leaf, ksuid.Nil)
	if err != nil {
		return nil, err
	}
	s.paths.Add(leaf, path)
	return path, nil
}

func (s *Store) PathRange(ctx context.Context, from, to ksuid.KSUID) ([]ksuid.KSUID, error) {
	var path []ksuid.KSUID
	for at := from; at != ksuid.Nil; {
		if cache, ok := s.paths.Get(at); ok {
			for _, id := range cache {
				path = append(path, id)
				if id == to {
					break
				}
			}
			break
		}
		path = append(path, at)
		o, err := s.Get(ctx, at)
		if err != nil {
			return nil, err
		}
		if at == to {
			break
		}
		at = o.Parent
	}
	return path, nil
}

func (s *Store) GetBytes(ctx context.Context, commit ksuid.KSUID) ([]byte, *Commit, error) {
	b, err := storage.Get(ctx, s.engine, s.pathOf(commit))
	if err != nil {
		return nil, nil, err
	}
	reader := bsupbytes.NewDeserializer(bytes.NewReader(b), ActionTypes)
	defer reader.Close()
	entry, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}
	first, ok := entry.(*Commit)
	if !ok {
		return nil, nil, fmt.Errorf("system error: first record of commit object is not a commit action: %s", s.pathOf(commit))
	}
	return b, first, nil
}

func (s *Store) ReadAll(ctx context.Context, commit, stop ksuid.KSUID) ([]byte, error) {
	var size int
	var buffers [][]byte
	for commit != ksuid.Nil && commit != stop {
		b, commitObject, err := s.GetBytes(ctx, commit)
		if err != nil {
			return nil, err
		}
		size += len(b)
		buffers = append(buffers, b)
		commit = commitObject.Parent
	}
	out := make([]byte, 0, size)
	for k := len(buffers) - 1; k >= 0; k-- {
		out = append(out, buffers[k]...)
	}
	return out, nil
}

func (s *Store) Open(ctx context.Context, commit, stop ksuid.KSUID) (io.Reader, error) {
	b, err := s.ReadAll(ctx, commit, stop)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

func (s *Store) OpenAsBSUP(ctx context.Context, zctx *super.Context, commit, stop ksuid.KSUID) (*bsupio.Reader, error) {
	r, err := s.Open(ctx, commit, stop)
	if err != nil {
		return nil, err
	}
	return bsupio.NewReader(zctx, r), nil
}

func (s *Store) OpenCommitLog(ctx context.Context, zctx *super.Context, commit, stop ksuid.KSUID) zio.Reader {
	return newLogReader(ctx, zctx, s, commit, stop)
}

// PatchOfCommit computes the snapshot at the parent of the indicated commit
// then computes the difference between that snapshot and the child commit,
// returning the difference as a patch.
func (s *Store) PatchOfCommit(ctx context.Context, commit ksuid.KSUID) (*Patch, error) {
	path, err := s.Path(ctx, commit)
	if err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return nil, errors.New("system error: no error on pathless commit")
	}
	var base *Snapshot
	if len(path) == 1 {
		// For first commit in branch, just create an empty base ...
		base = NewSnapshot()
	} else {
		parent := path[1]
		base, err = s.Snapshot(ctx, parent)
		if err != nil {
			return nil, err
		}
	}
	patch := NewPatch(base)
	object, err := s.Get(ctx, commit)
	if err != nil {
		return nil, err
	}
	for _, action := range object.Actions {
		if err := PlayAction(patch, action); err != nil {
			return nil, err
		}
	}
	return patch, nil
}

func (s *Store) PatchOfPath(ctx context.Context, base *Snapshot, baseID, commit ksuid.KSUID) (*Patch, error) {
	path, err := s.PathRange(ctx, commit, baseID)
	if err != nil {
		return nil, err
	}
	patch := NewPatch(base)
	if len(path) < 2 {
		// There are no changes past the base.  Return the empty patch.
		return patch, nil
	}
	// Play objects in forward order skipping over the last path element
	// as that is the base and the difference is relative to it.
	for k := len(path) - 2; k >= 0; k-- {
		o, err := s.Get(ctx, path[k])
		if err != nil {
			return nil, err
		}
		for _, action := range o.Actions {
			if err := PlayAction(patch, action); err != nil {
				return nil, err
			}
		}
	}
	return patch, nil
}

// Vacuumable returns the set of data.Objects in the path of leaf that are not referenced
// by the leaf's snapshot.
func (s *Store) Vacuumable(ctx context.Context, leaf ksuid.KSUID, out chan<- *data.Object) error {
	snap, err := s.Snapshot(ctx, leaf)
	if err != nil {
		return err
	}
	for at := leaf; at != ksuid.Nil; {
		o, err := s.Get(ctx, at)
		if err != nil {
			return nil
		}
		at = o.Parent
		if o.Commit == leaf {
			// skip the leaf commit.
			continue
		}
		for _, action := range o.Actions {
			switch a := action.(type) {
			case *Add:
				if !snap.Exists(a.Object.ID) {
					select {
					case out <- &a.Object:
					case <-ctx.Done():
					}
				}
			// XXX Support *AddVector, but currently Vector only has an ID and descriptive object.
			default:
				continue
			}
		}
	}
	return nil
}
