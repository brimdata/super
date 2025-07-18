package lake

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/brimdata/super"
	"github.com/brimdata/super/bsupbytes"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/lake/branches"
	"github.com/brimdata/super/lake/data"
	"github.com/brimdata/super/lake/pools"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio/bsupio"
	arc "github.com/hashicorp/golang-lru/arc/v2"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
)

const (
	Version         = 4
	PoolsTag        = "pools"
	LakeMagicFile   = "lake.bsup"
	LakeMagicString = "ZED LAKE"
)

var (
	ErrExist    = errors.New("lake already exists")
	ErrNotExist = errors.New("lake does not exist")
)

// The Root of the lake represents the path prefix and configuration state
// for all of the data pools in the lake.
type Root struct {
	engine storage.Engine
	logger *zap.Logger
	path   *storage.URI

	poolCache *arc.ARCCache[ksuid.KSUID, *Pool]
	pools     *pools.Store
	vCache    *vcache.Cache
}

type LakeMagic struct {
	Magic   string `super:"magic"`
	Version int    `super:"version"`
}

func newRoot(engine storage.Engine, logger *zap.Logger, path *storage.URI) *Root {
	poolCache, err := arc.NewARC[ksuid.KSUID, *Pool](1024)
	if err != nil {
		panic(err)
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Root{
		engine:    engine,
		logger:    logger,
		path:      path,
		poolCache: poolCache,
		vCache:    vcache.NewCache(engine),
	}
}

func Open(ctx context.Context, engine storage.Engine, logger *zap.Logger, path *storage.URI) (*Root, error) {
	r := newRoot(engine, logger, path)
	if err := r.loadConfig(ctx); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = fmt.Errorf("%s: %w", path, ErrNotExist)
		}
		return nil, err
	}
	return r, nil
}

func Create(ctx context.Context, engine storage.Engine, logger *zap.Logger, path *storage.URI) (*Root, error) {
	r := newRoot(engine, logger, path)
	if err := r.loadConfig(ctx); err == nil {
		return nil, fmt.Errorf("%s: %w", path, ErrExist)
	}
	if err := r.createConfig(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

func CreateOrOpen(ctx context.Context, engine storage.Engine, logger *zap.Logger, path *storage.URI) (*Root, error) {
	r, err := Open(ctx, engine, logger, path)
	if errors.Is(err, ErrNotExist) {
		return Create(ctx, engine, logger, path)
	}
	return r, err
}

func (r *Root) createConfig(ctx context.Context) error {
	poolPath := r.path.JoinPath(PoolsTag)
	var err error
	r.pools, err = pools.CreateStore(ctx, r.engine, r.logger, poolPath)
	if err != nil {
		return err
	}
	return r.writeLakeMagic(ctx)
}

func (r *Root) loadConfig(ctx context.Context) error {
	if err := r.readLakeMagic(ctx); err != nil {
		return err
	}
	poolPath := r.path.JoinPath(PoolsTag)
	var err error
	r.pools, err = pools.OpenStore(ctx, r.engine, r.logger, poolPath)
	if err != nil {
		return err
	}
	return err
}

func (r *Root) writeLakeMagic(ctx context.Context) error {
	if err := r.readLakeMagic(ctx); err == nil {
		return errors.New("lake already exists")
	}
	magic := &LakeMagic{
		Magic:   LakeMagicString,
		Version: Version,
	}
	serializer := bsupbytes.NewSerializer()
	serializer.Decorate(sup.StylePackage)
	if err := serializer.Write(magic); err != nil {
		return err
	}
	if err := serializer.Close(); err != nil {
		return err
	}
	path := r.path.JoinPath(LakeMagicFile)
	err := r.engine.PutIfNotExists(ctx, path, serializer.Bytes())
	if err == storage.ErrNotSupported {
		//XXX workaround for now: see issue #2686
		reader := bytes.NewReader(serializer.Bytes())
		err = storage.Put(ctx, r.engine, path, reader)
	}
	return err
}

func (r *Root) readLakeMagic(ctx context.Context) error {
	path := r.path.JoinPath(LakeMagicFile)
	reader, err := r.engine.Get(ctx, path)
	if err != nil {
		return err
	}
	zr := bsupio.NewReader(super.NewContext(), reader)
	defer zr.Close()
	val, err := zr.Read()
	if err != nil {
		return err
	}
	last, err := zr.Read()
	if err != nil {
		return err
	}
	if last != nil {
		return fmt.Errorf("corrupt lake version file: more than one Zed value at %s", sup.String(last))
	}
	var magic LakeMagic
	if err := sup.UnmarshalBSUP(*val, &magic); err != nil {
		return fmt.Errorf("corrupt lake version file: %w", err)
	}
	if magic.Magic != LakeMagicString {
		return fmt.Errorf("corrupt lake version file: magic %q should be %q", magic.Magic, LakeMagicString)
	}
	if magic.Version != Version {
		return fmt.Errorf("unsupported lake version: found version %d while expecting %d", magic.Version, Version)
	}
	return nil
}

func (r *Root) BatchifyPools(ctx context.Context, sctx *super.Context, f expr.Evaluator) ([]super.Value, error) {
	m := sup.NewBSUPMarshalerWithContext(sctx)
	m.Decorate(sup.StylePackage)
	pools, err := r.ListPools(ctx)
	if err != nil {
		return nil, err
	}
	var vals []super.Value
	for k := range pools {
		rec, err := m.Marshal(&pools[k])
		if err != nil {
			return nil, err
		}
		if filter(sctx, rec, f) {
			vals = append(vals, rec)
		}
	}
	return vals, nil
}

func (r *Root) BatchifyBranches(ctx context.Context, sctx *super.Context, f expr.Evaluator) ([]super.Value, error) {
	m := sup.NewBSUPMarshalerWithContext(sctx)
	m.Decorate(sup.StylePackage)
	poolRefs, err := r.ListPools(ctx)
	if err != nil {
		return nil, err
	}
	var vals []super.Value
	for k := range poolRefs {
		pool, err := r.openPool(ctx, &poolRefs[k])
		if err != nil {
			// We could have race here because a pool got deleted
			// while we looped so we check and continue.
			if errors.Is(err, pools.ErrNotFound) {
				continue
			}
			return nil, err
		}
		vals, err = pool.BatchifyBranches(ctx, sctx, vals, m, f)
		if err != nil {
			return nil, err
		}
	}
	return vals, nil
}

type BranchMeta struct {
	Pool   pools.Config    `super:"pool"`
	Branch branches.Config `super:"branch"`
}

func (r *Root) ListPools(ctx context.Context) ([]pools.Config, error) {
	return r.pools.All(ctx)
}

func (r *Root) PoolID(ctx context.Context, poolName string) (ksuid.KSUID, error) {
	if poolName == "" {
		return ksuid.Nil, errors.New("no pool name given")
	}
	poolRef := r.pools.LookupByName(ctx, poolName)
	if poolRef == nil {
		return ksuid.Nil, fmt.Errorf("%s: %w", poolName, pools.ErrNotFound)
	}
	return poolRef.ID, nil
}

func (r *Root) CommitObject(ctx context.Context, poolID ksuid.KSUID, branchName string) (ksuid.KSUID, error) {
	pool, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return ksuid.Nil, err
	}
	branchRef, err := pool.LookupBranchByName(ctx, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	return branchRef.Commit, nil
}

func (r *Root) SortKeys(ctx context.Context, src dag.Op) order.SortKeys {
	switch src := src.(type) {
	case *dag.Lister:
		if config, err := r.pools.LookupByID(ctx, src.Pool); err == nil {
			return config.SortKeys
		}
	case *dag.SeqScan:
		if config, err := r.pools.LookupByID(ctx, src.Pool); err == nil {
			return config.SortKeys
		}
	case *dag.PoolScan:
		if config, err := r.pools.LookupByID(ctx, src.ID); err == nil {
			return config.SortKeys
		}
	case *dag.CommitMetaScan:
		if src.Tap {
			if config, err := r.pools.LookupByID(ctx, src.Pool); err == nil {
				return config.SortKeys
			}
		}
	}
	return nil
}

func (r *Root) OpenPool(ctx context.Context, id ksuid.KSUID) (*Pool, error) {
	config, err := r.pools.LookupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.openPool(ctx, config)
}

func (r *Root) openPool(ctx context.Context, config *pools.Config) (*Pool, error) {
	if p, ok := r.poolCache.Get(config.ID); ok {
		// The cached pool's config may be outdated, so rather than
		// return the pool directly, we return a copy whose config we
		// can safely update without locking.
		p := *p
		p.Config = *config
		return &p, nil
	}
	p, err := OpenPool(ctx, r.engine, r.logger, r.path, config)
	if err != nil {
		return nil, err
	}
	r.poolCache.Add(config.ID, p)
	return p, nil
}

func (r *Root) RenamePool(ctx context.Context, id ksuid.KSUID, newName string) error {
	return r.pools.Rename(ctx, id, newName)
}

func (r *Root) CreatePool(ctx context.Context, name string, sortKeys order.SortKeys, seekStride int, thresh int64) (*Pool, error) {
	if name == "HEAD" {
		return nil, fmt.Errorf("pool cannot be named %q", name)
	}
	if r.pools.LookupByName(ctx, name) != nil {
		return nil, fmt.Errorf("%s: %w", name, pools.ErrExists)
	}
	if thresh == 0 {
		thresh = data.DefaultThreshold
	}
	if len(sortKeys) > 1 {
		return nil, errors.New("multiple pool keys not supported")
	}
	config := pools.NewConfig(name, sortKeys, thresh, seekStride)
	if err := CreatePool(ctx, r.engine, r.logger, r.path, config); err != nil {
		return nil, err
	}
	pool, err := r.openPool(ctx, config)
	if err != nil {
		RemovePool(ctx, r.engine, r.path, config)
		return nil, err
	}
	if err := r.pools.Add(ctx, config); err != nil {
		RemovePool(ctx, r.engine, r.path, config)
		return nil, err
	}
	return pool, nil
}

// RemovePool deletes a pool from the configuration journal and deletes all
// data associated with the pool.
func (r *Root) RemovePool(ctx context.Context, id ksuid.KSUID) error {
	config, err := r.pools.LookupByID(ctx, id)
	if err != nil {
		return err
	}
	if err := r.pools.Remove(ctx, *config); err != nil {
		return err
	}
	// This pool might be cached on other cluster nodes, but that's fine.
	// With no entry in the pool store, it will be inaccessible and
	// eventually evicted by the cache's LRU algorithm.
	r.poolCache.Remove(config.ID)
	return RemovePool(ctx, r.engine, r.path, config)
}

func (r *Root) CreateBranch(ctx context.Context, poolID ksuid.KSUID, name string, parent ksuid.KSUID) (*branches.Config, error) {
	config, err := r.pools.LookupByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	return CreateBranch(ctx, r.engine, r.logger, r.path, config, name, parent)
}

func (r *Root) RemoveBranch(ctx context.Context, poolID ksuid.KSUID, name string) error {
	pool, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return err
	}
	return pool.removeBranch(ctx, name)
}

// MergeBranch merges the indicated branch into its parent returning the
// commit tag of the new commit into the parent branch.
func (r *Root) MergeBranch(ctx context.Context, poolID ksuid.KSUID, childBranch, parentBranch, author, message string) (ksuid.KSUID, error) {
	pool, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return ksuid.Nil, err
	}
	child, err := pool.OpenBranchByName(ctx, childBranch)
	if err != nil {
		return ksuid.Nil, err
	}
	parent, err := pool.OpenBranchByName(ctx, parentBranch)
	if err != nil {
		return ksuid.Nil, err
	}
	return child.mergeInto(ctx, parent, author, message)
}

func (r *Root) Revert(ctx context.Context, poolID ksuid.KSUID, branchName string, commitID ksuid.KSUID, author, message string) (ksuid.KSUID, error) {
	pool, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return ksuid.Nil, err
	}
	branch, err := pool.OpenBranchByName(ctx, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	return branch.Revert(ctx, commitID, author, message)
}

func (r *Root) Open(context.Context, *super.Context, string, string, zbuf.Pushdown) (zbuf.Puller, error) {
	return nil, errors.New("cannot use 'file' or 'http' source in a lake query")
}

func (r *Root) VectorCache() *vcache.Cache {
	return r.vCache
}
