package api

import (
	"context"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/lake/index"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/segmentio/ksuid"
)

type Interface interface {
	Query(ctx context.Context, d driver.Driver, src string, srcfiles ...string) (zbuf.ScannerStats, error)

	CreatePool(context.Context, string, order.Layout, int64) (*lake.PoolConfig, error)
	RemovePool(context.Context, ksuid.KSUID) error
	RenamePool(context.Context, ksuid.KSUID, string) error
	LookupPool(context.Context, string) (*lake.PoolConfig, error)

	AddIndexRules(context.Context, []index.Rule) error
	DeleteIndexRules(context.Context, []ksuid.KSUID) ([]index.Rule, error)
	ApplyIndexRules(ctx context.Context, rule string, pool ksuid.KSUID, ids []ksuid.KSUID) (ksuid.KSUID, error)

	// Data operations
	Add(ctx context.Context, pool ksuid.KSUID, r zio.Reader, commit *api.CommitRequest) (ksuid.KSUID, error)
	Delete(ctx context.Context, pool ksuid.KSUID, tags []ksuid.KSUID, commit *api.CommitRequest) (ksuid.KSUID, error)
	Commit(ctx context.Context, pool ksuid.KSUID, id ksuid.KSUID, commit api.CommitRequest) error
	Squash(ctx context.Context, pool ksuid.KSUID, ids []ksuid.KSUID) (ksuid.KSUID, error)

	// XXX ScanStaging will go away with issue #2626
	// this way when the log converts to a sub-pool the API here is the same...
	ScanStaging(ctx context.Context, pool ksuid.KSUID, w zio.Writer, ids []ksuid.KSUID) error
}

func ScanIndexRules(ctx context.Context, api Interface, d driver.Driver) error {
	_, err := api.Query(ctx, d, "from [index_rules]")
	return err
}
