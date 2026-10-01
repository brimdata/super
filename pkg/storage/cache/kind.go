// Package cache contains facilities for caching immutable files, typically
// for a cloud object store.
package cache

import (
	"fmt"

	"github.com/superdb/super/pkg/storage"
)

type Cacheable func(*storage.URI) bool

type Kind string

const (
	KindNone  Kind = "none"
	KindLocal Kind = "local"
	KindRedis Kind = "redis"
)

func (k *Kind) Set(s string) error {
	switch s {
	case "none", "":
		*k = KindNone
	case "local":
		*k = KindLocal
	case "redis":
		*k = KindRedis
	default:
		return fmt.Errorf("unknown immutable cache kind: %q", s)
	}
	return nil
}

func (k Kind) String() string {
	return string(k)
}
