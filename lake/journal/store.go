package journal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zngbytes"
	"github.com/brimdata/zed/zson"
)

const maxRetries = 10

var (
	ErrRetriesExceeded = fmt.Errorf("config journal unavailable after %d attempts", maxRetries)
	ErrKeyExists       = errors.New("key already exists")
	ErrNoSuchKey       = errors.New("no such key")
	ErrConstraint      = errors.New("constraint failed")
)

type Store struct {
	journal     *Queue
	style       zson.TypeStyle
	unmarshaler *zson.UnmarshalZNGContext
	table       map[string]Entry
	at          ID
	loadTime    time.Time
}

type Entry interface {
	Key() string
}

type Add struct {
	Entry `zed:"entry"`
}

type Update struct {
	Entry `zed:"entry"`
}

type Delete struct {
	EntryKey string `zed:"entry_key"`
}

func (d *Delete) Key() string {
	return d.EntryKey
}

func newStore(path *storage.URI, entryTypes ...interface{}) *Store {
	u := zson.NewZNGUnmarshaler()
	u.Bind(Add{}, Delete{}, Update{})
	u.Bind(entryTypes...)
	return &Store{
		style:       zson.StylePackage,
		unmarshaler: u,
	}
}

func (s *Store) Decorate(style zson.TypeStyle) {
	s.style = style
}

func (s *Store) load(ctx context.Context) error {
	head, err := s.journal.ReadHead(ctx)
	if err != nil {
		return err
	}
	if head == s.at {
		return nil
	}
	r, err := s.journal.OpenAsZNG(ctx, zed.NewContext(), head, 0)
	if err != nil {
		return err
	}
	table := make(map[string]Entry)
	for {
		rec, err := r.Read()
		if err != nil {
			return err
		}
		if rec == nil {
			s.table = table
			s.loadTime = time.Now()
			s.at = head
			return nil

		}
		var e Entry
		if err := s.unmarshaler.Unmarshal(*rec, &e); err != nil {
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

func (s *Store) stale() bool {
	if s.loadTime.IsZero() {
		return true
	}
	return time.Now().Sub(s.loadTime) > time.Second
}

func (s *Store) Keys(ctx context.Context, key string) ([]string, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(s.table))
	for key := range s.table {
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *Store) Values(ctx context.Context) ([]interface{}, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	vals := make([]interface{}, 0, len(s.table))
	for _, val := range s.table {
		vals = append(vals, val)
	}
	return vals, nil
}

func (s *Store) All(ctx context.Context) ([]Entry, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(s.table))
	for _, e := range s.table {
		entries = append(entries, e)
	}
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
	val, ok := s.table[key]
	if !ok {
		if fresh {
			return nil, ErrNoSuchKey
		}
		// If we didn't load the table, try loading it
		// then re-checking for the key.
		if err := s.load(ctx); err != nil {
			return nil, err
		}
		val, ok = s.table[key]
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
	serializer := zngbytes.NewSerializer()
	serializer.Decorate(s.style)
	for _, e := range entries {
		if err := serializer.Write(e); err != nil {
			return err
		}
	}
	if err := serializer.Close(); err != nil {
		return err
	}
	for attempts := 0; attempts < maxRetries; attempts++ {
		if err := s.load(ctx); err != nil {
			return err
		}
		if err := fn(); err != nil {
			return err
		}
		if err := s.journal.CommitAt(ctx, s.at, serializer.Bytes()); err != nil {
			if os.IsExist(err) {
				time.Sleep(time.Millisecond)
				continue
			}
			return err
		}
		// Force a reload after a change.
		s.at = Nil
		return nil
	}
	return ErrRetriesExceeded
}

func OpenStore(ctx context.Context, engine storage.Engine, path *storage.URI, keyTypes ...interface{}) (*Store, error) {
	s := newStore(path, keyTypes...)
	var err error
	s.journal, err = Open(ctx, engine, path)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func CreateStore(ctx context.Context, engine storage.Engine, path *storage.URI, keyTypes ...interface{}) (*Store, error) {
	s := newStore(path, keyTypes...)
	j, err := Create(ctx, engine, path, Nil)
	if err != nil {
		return nil, err
	}
	s.journal = j
	return s, nil
}
