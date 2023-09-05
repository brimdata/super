package lakemanage

import (
	"context"

	"github.com/brimdata/zed/api"
	lakeapi "github.com/brimdata/zed/lake/api"
	"github.com/brimdata/zed/lake/pools"
	"github.com/brimdata/zed/lakeparse"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type branch struct {
	config PoolConfig
	lake   lakeapi.Interface
	logger *zap.Logger
	pool   *pools.Config
}

func newBranch(c Config, pool *pools.Config, lake lakeapi.Interface, logger *zap.Logger) *branch {
	config := c.poolConfig(pool)
	logger = logger.Named("pool").With(
		zap.String("name", pool.Name),
		zap.Stringer("id", pool.ID),
		zap.String("branch", config.Branch),
		zap.Duration("interval", config.interval()),
		zap.Bool("vectors", config.Vectors),
	)
	if config.interval() == 0 {
		logger.Info("Manage disabled for branch")
		return nil
	}
	return &branch{
		config: config,
		lake:   lake,
		logger: logger,
		pool:   pool,
	}
}

func (b *branch) run(ctx context.Context) error {
	b.logger.Debug("compaction started")
	head := lakeparse.Commitish{Pool: b.pool.Name, Branch: b.config.Branch}
	it, err := newObjectIterator(ctx, b.lake, &head)
	if err != nil {
		return err
	}
	defer it.close()
	runCh := make(chan []ksuid.KSUID)
	vecCh := make(chan ksuid.KSUID)
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		err := scan(ctx, it, b.pool, runCh, vecCh)
		close(runCh)
		close(vecCh)
		return err
	})
	var found int
	var compacted int
	var vectors int
	group.Go(func() error {
		for run := range runCh {
			commit, err := b.lake.Compact(ctx, b.pool.ID, b.config.Branch, run, b.config.Vectors, api.CommitMessage{})
			if err != nil {
				return err
			}
			found++
			compacted += len(run)
			b.logger.Debug("compacted", zap.Stringer("commit", commit), zap.Int("objects_compacted", len(run)))
		}
		return nil
	})
	group.Go(func() error {
		var oids []ksuid.KSUID
		for oid := range vecCh {
			if b.config.Vectors {
				oids = append(oids, oid)
			}
		}
		if len(oids) == 0 {
			return nil
		}
		_, err := b.lake.AddVectors(ctx, head.Pool, head.Branch, oids, api.CommitMessage{})
		if err == nil {
			vectors += len(oids)
		}
		return err
	})
	err = group.Wait()
	b.logger.Info("compaction completed",
		zap.Int("runs_found", found),
		zap.Int("objects_compacted", compacted),
		zap.Int("vectors_created", vectors),
	)
	return err
}
