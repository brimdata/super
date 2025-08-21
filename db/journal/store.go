package journal

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/bsupbytes"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
	"go.uber.org/zap"
)

const maxRetries = 10

var (
	ErrRetriesExceeded = fmt.Errorf("config journal unavailable after %d attempts", maxRetries)
	ErrKeyExists       = errors.New("key already exists")
	ErrNoSuchKey       = errors.New("no such key")
	ErrConstraint      = errors.New("constraint failed")
)

type Store struct {
	journal  *Queue
	logger   *zap.Logger
	keyTypes []any

	mu       sync.RWMutex // Protects everything below.
	table    map[string]Entry
	at       ID
	loadTime time.Time
}

type Entry interface {
	Key() string
}

type Add struct {
	Entry `super:"entry"`
}

type Update struct {
	Entry `super:"entry"`
}

type Delete struct {
	EntryKey string `super:"entry_key"`
}

func (d *Delete) Key() string {
	return d.EntryKey
}

func CreateStore(ctx context.Context, engine storage.Engine, logger *zap.Logger, path *storage.URI, keyTypes ...any) (*Store, error) {
	journal, err := Create(ctx, engine, path, Nil)
	if err != nil {
		return nil, err
	}
	return newStore(journal, logger, keyTypes...), nil
}

func OpenStore(ctx context.Context, engine storage.Engine, logger *zap.Logger, path *storage.URI, keyTypes ...any) (*Store, error) {
	journal, err := Open(ctx, engine, path)
	if err != nil {
		return nil, err
	}
	return newStore(journal, logger, keyTypes...), nil
}

func newStore(journal *Queue, logger *zap.Logger, keyTypes ...any) *Store {
	return &Store{
		journal:  journal,
		logger:   logger.Named("journal"),
		keyTypes: append([]any{Add{}, Delete{}, Update{}}, keyTypes...),
	}
}

func (s *Store) load(ctx context.Context) error {
	head, err := s.journal.ReadHead(ctx)
	if err != nil {
		return err
	}
	s.mu.RLock()
	current := s.at
	s.mu.RUnlock()
	if head == current {
		return nil
	}
	unmarshaler := sup.NewBSUPUnmarshaler()
	unmarshaler.Bind(s.keyTypes...)
	at, table, err := s.getSnapshot(ctx, unmarshaler)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.logger.Error("Loading snapshot", zap.Error(err))
	}
	r, err := s.journal.OpenAsBSUP(ctx, super.NewContext(), head, at)
	if err != nil {
		return err
	}
	defer r.Close()
	for {
		val, err := r.Read()
		if err != nil {
			return err
		}
		if val == nil {
			now := time.Now()
			s.mu.Lock()
			s.table = table
			s.at = head
			s.loadTime = now
			s.mu.Unlock()
			// Reduce the amount of times we write snapshots to disk by only writing when there are
			// more than 10 new entries since the last snapshot.
			if head-at > 10 {
				if err := s.putSnapshot(ctx, head, table); err != nil {
					s.logger.Error("Storing snapshot", zap.Error(err))
				}
			}
			return nil
		}
		var e Entry
		if err := unmarshaler.Unmarshal(*val, &e); err != nil {
			return err
		}
		switch e := e.(type) {
		case *Add:
			table[e.Entry.Key()] = e.Entry
		case *Update:
			key := e.Key()
			if _, ok := table[key]; !ok {
				return fmt.Errorf("update to non-existent key in journal store: %T", key)
			}
			table[key] = e.Entry
		case *Delete:
			delete(table, e.EntryKey)
		default:
			return fmt.Errorf("unknown type in journal store: %T", e)
		}
	}
}

func (s *Store) getSnapshot(ctx context.Context, unmarshaler *sup.UnmarshalBSUPContext) (ID, map[string]Entry, error) {
	table := make(map[string]Entry)
	r, err := s.journal.engine.Get(ctx, s.snapshotURI())
	if err != nil {
		return Nil, table, err
	}
	defer r.Close()
	zr := bsupio.NewReader(super.NewContext(), r)
	defer zr.Close()
	val, err := zr.Read()
	if val == nil || err != nil {
		return Nil, table, err
	}
	if val.Type().ID() != super.IDUint64 {
		return Nil, table, errors.New("corrupted journal snapshot")
	}
	at := ID(val.Uint())
	for {
		val, err := zr.Read()
		if val == nil || err != nil {
			return at, table, err
		}
		var e Entry
		if err := unmarshaler.Unmarshal(*val, &e); err != nil {
			return at, nil, err
		}
		table[e.Key()] = e
	}
}

func (s *Store) putSnapshot(ctx context.Context, at ID, table map[string]Entry) error {
	// XXX This needs to be an atomic write for file systems: brimdata/super#4277.
	w, err := s.journal.engine.Put(ctx, s.snapshotURI())
	if err != nil {
		return err
	}
	zw := bsupio.NewWriter(w)
	defer zw.Close()
	if err := zw.Write(super.NewUint64(uint64(at))); err != nil {
		return err
	}
	marshaler := sup.NewBSUPMarshaler()
	marshaler.Decorate(sup.StylePackage)
	for _, entry := range table {
		val, err := marshaler.Marshal(entry)
		if err != nil {
			return err
		}
		if err := zw.Write(val); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) snapshotURI() *storage.URI {
	return s.journal.path.JoinPath(fmt.Sprintf("snap.%s", ext))
}

func (s *Store) stale() bool {
	s.mu.RLock()
	loadTime := s.loadTime
	s.mu.RUnlock()
	return time.Since(loadTime) > time.Second
}

func (s *Store) Keys(ctx context.Context, key string) ([]string, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	keys := make([]string, 0, len(s.table))
	for key := range s.table {
		keys = append(keys, key)
	}
	s.mu.RUnlock()
	return keys, nil
}

func (s *Store) Values(ctx context.Context) ([]any, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	vals := make([]any, 0, len(s.table))
	for _, val := range s.table {
		vals = append(vals, val)
	}
	s.mu.RUnlock()
	return vals, nil
}

func (s *Store) All(ctx context.Context) ([]Entry, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	entries := make([]Entry, 0, len(s.table))
	for _, e := range s.table {
		entries = append(entries, e)
	}
	s.mu.RUnlock()
	return entries, nil
}

func (s *Store) Lookup(ctx context.Context, key string) (Entry, error) {
	var fresh bool
	if s.stale() {
		if err := s.load(ctx); err != nil {
			return nil, err
		}
		fresh = true
	}
	s.mu.RLock()
	val, ok := s.table[key]
	s.mu.RUnlock()
	if !ok {
		if fresh {
			return nil, ErrNoSuchKey
		}
		// If we didn't load the table, try loading it
		// then re-checking for the key.
		if err := s.load(ctx); err != nil {
			return nil, err
		}
		s.mu.RLock()
		val, ok = s.table[key]
		s.mu.RUnlock()
		if !ok {
			return nil, ErrNoSuchKey
		}
	}
	return val, nil
}

func (s *Store) Insert(ctx context.Context, e Entry) error {
	return s.commit(ctx, func() error {
		if _, ok := s.table[e.Key()]; ok {
			return ErrKeyExists
		}
		return nil
	}, &Add{e})
}

func (s *Store) Move(ctx context.Context, oldKey string, newEntry Entry) error {
	return s.commit(ctx, func() error {
		if _, ok := s.table[oldKey]; !ok {
			return ErrNoSuchKey
		}
		if _, ok := s.table[newEntry.Key()]; ok {
			return ErrKeyExists
		}
		return nil
	}, &Delete{oldKey}, &Add{newEntry})
}

type Constraint func(Entry) bool

func (s *Store) Delete(ctx context.Context, key string, c Constraint) error {
	return s.commitWithConstraint(ctx, key, c, &Delete{key})
}

func (s *Store) Update(ctx context.Context, e Entry, c Constraint) error {
	return s.commitWithConstraint(ctx, e.Key(), c, &Update{e})
}

func (s *Store) commitWithConstraint(ctx context.Context, key string, c Constraint, e Entry) error {
	return s.commit(ctx, func() error {
		oldEntry, ok := s.table[key]
		if !ok {
			return ErrNoSuchKey
		}
		if c != nil && !c(oldEntry) {
			return ErrConstraint
		}
		return nil
	}, e)
}

func (s *Store) commit(ctx context.Context, fn func() error, entries ...Entry) error {
	serializer := bsupbytes.NewSerializer()
	serializer.Decorate(sup.StylePackage)
	for _, e := range entries {
		if err := serializer.Write(e); err != nil {
			return err
		}
	}
	if err := serializer.Close(); err != nil {
		return err
	}
	for range maxRetries {
		if err := s.load(ctx); err != nil {
			return err
		}
		s.mu.RLock()
		at := s.at
		err := fn()
		s.mu.RUnlock()
		if err != nil {
			return err
		}
		if err := s.journal.CommitAt(ctx, at, serializer.Bytes()); err != nil {
			if os.IsExist(err) {
				time.Sleep(time.Millisecond)
				continue
			}
			return err
		}
		// Force a reload after a change.
		s.mu.Lock()
		s.at = Nil
		s.mu.Unlock()
		return nil
	}
	return ErrRetriesExceeded
}
