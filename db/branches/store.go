package branches

import (
	"context"
	"errors"
	"fmt"

	"github.com/brimdata/super/db/journal"
	"github.com/brimdata/super/pkg/storage"
	"go.uber.org/zap"
)

var (
	ErrExists   = errors.New("branch already exists")
	ErrNotFound = errors.New("branch not found")
)

type Store struct {
	store *journal.Store
}

func CreateStore(ctx context.Context, engine storage.Engine, logger *zap.Logger, path *storage.URI) (*Store, error) {
	store, err := journal.CreateStore(ctx, engine, logger, path, Config{})
	if err != nil {
		return nil, err
	}
	return &Store{store}, nil
}

func OpenStore(ctx context.Context, engine storage.Engine, logger *zap.Logger, path *storage.URI) (*Store, error) {
	store, err := journal.OpenStore(ctx, engine, logger, path, Config{})
	if err != nil {
		return nil, err
	}
	return &Store{store}, nil
}

func (s *Store) All(ctx context.Context) ([]Config, error) {
	entries, err := s.store.All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]Config, 0, len(entries))
	for _, entry := range entries {
		branch, ok := entry.(*Config)
		if !ok {
			return nil, errors.New("corrupt branch config journal")
		}
		list = append(list, *branch)
	}
	return list, nil
}

func (s *Store) LookupByName(ctx context.Context, name string) (*Config, error) {
	list, _ := s.All(ctx)
	for k, config := range list {
		if config.Name == name {
			return &list[k], nil
		}
	}
	return nil, fmt.Errorf("%q: %w", name, ErrNotFound)
}

func (s *Store) Add(ctx context.Context, config *Config) error {
	return s.store.Insert(ctx, config)
}

func (s *Store) Update(ctx context.Context, config *Config, c journal.Constraint) error {
	return s.store.Update(ctx, config, c)
}

// Remove deletes a branch from the configuration journal.
// We make sure the last commit is the same as the reference config;
// otherwise, there was a race and someone did something with this
// branch in the meantime so we abort.
func (s *Store) Remove(ctx context.Context, config Config) error {
	err := s.store.Delete(ctx, config.Name, func(v journal.Entry) bool {
		p, ok := v.(*Config)
		if !ok {
			return false
		}
		return p.Commit == config.Commit
	})
	if err != nil {
		if err == journal.ErrNoSuchKey {
			return fmt.Errorf("%s: %w", config.Name, ErrNotFound)
		}
		if err == journal.ErrConstraint {
			return fmt.Errorf("%q: branch at commit %s operated on during removal.. aborted", config.Name, config.Commit)
		}
		return err
	}
	return nil
}
