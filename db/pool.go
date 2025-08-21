package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/db/branches"
	"github.com/brimdata/super/db/commits"
	"github.com/brimdata/super/db/data"
	"github.com/brimdata/super/db/pools"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	DataTag     = "data"
	BranchesTag = "branches"
	CommitsTag  = "commits"
)

type Pool struct {
	pools.Config
	engine   storage.Engine
	Path     *storage.URI
	DataPath *storage.URI
	branches *branches.Store
	commits  *commits.Store
}

func CreatePool(ctx context.Context, engine storage.Engine, logger *zap.Logger, root *storage.URI, config *pools.Config) error {
	poolPath := config.Path(root)
	// branchesPath is the path to the kvs journal of BranchConfigs
	// for the pool while the commit log is stored in <pool-id>/<branch-id>.
	branchesPath := poolPath.JoinPath(BranchesTag)
	// create the branches journal store
	_, err := branches.CreateStore(ctx, engine, logger, branchesPath)
	if err != nil {
		return err
	}
	// create the main branch in the branches journal store.  The parent
	// commit object of the initial main branch is ksuid.Nil.
	_, err = CreateBranch(ctx, engine, logger, root, config, "main", ksuid.Nil)
	return err
}

func CreateBranch(ctx context.Context, engine storage.Engine, logger *zap.Logger, root *storage.URI, poolConfig *pools.Config, name string, parent ksuid.KSUID) (*branches.Config, error) {
	poolPath := poolConfig.Path(root)
	branchesPath := poolPath.JoinPath(BranchesTag)
	store, err := branches.OpenStore(ctx, engine, logger, branchesPath)
	if err != nil {
		return nil, err
	}
	if _, err := store.LookupByName(ctx, name); err == nil {
		return nil, fmt.Errorf("%s/%s: %w", poolConfig.Name, name, branches.ErrExists)
	}
	branchConfig := branches.NewConfig(name, parent)
	if err := store.Add(ctx, branchConfig); err != nil {
		return nil, err
	}
	return branchConfig, err
}

func OpenPool(ctx context.Context, engine storage.Engine, logger *zap.Logger, root *storage.URI, config *pools.Config) (*Pool, error) {
	path := config.Path(root)
	branchesPath := path.JoinPath(BranchesTag)
	branches, err := branches.OpenStore(ctx, engine, logger, branchesPath)
	if err != nil {
		return nil, err
	}
	commitsPath := path.JoinPath(CommitsTag)
	commits, err := commits.OpenStore(engine, logger, commitsPath)
	if err != nil {
		return nil, err
	}
	return &Pool{
		Config:   *config,
		engine:   engine,
		Path:     path,
		DataPath: DataPath(path),
		branches: branches,
		commits:  commits,
	}, nil
}

func RemovePool(ctx context.Context, engine storage.Engine, root *storage.URI, config *pools.Config) error {
	return engine.DeleteByPrefix(ctx, config.Path(root))
}

func (p *Pool) removeBranch(ctx context.Context, name string) error {
	config, err := p.branches.LookupByName(ctx, name)
	if err != nil {
		return err
	}
	return p.branches.Remove(ctx, *config)
}

func (p *Pool) Snapshot(ctx context.Context, commit ksuid.KSUID) (commits.View, error) {
	return p.commits.Snapshot(ctx, commit)
}

func (p *Pool) OpenCommitLog(ctx context.Context, sctx *super.Context, commit ksuid.KSUID) sio.Reader {
	return p.commits.OpenCommitLog(ctx, sctx, commit, ksuid.Nil)
}

func (p *Pool) OpenCommitLogAsBSUP(ctx context.Context, sctx *super.Context, commit ksuid.KSUID) (*bsupio.Reader, error) {
	return p.commits.OpenAsBSUP(ctx, sctx, commit, ksuid.Nil)
}

func (p *Pool) Storage() storage.Engine {
	return p.engine
}

func (p *Pool) ListBranches(ctx context.Context) ([]branches.Config, error) {
	return p.branches.All(ctx)
}

func (p *Pool) LookupBranchByName(ctx context.Context, name string) (*branches.Config, error) {
	return p.branches.LookupByName(ctx, name)
}

func (p *Pool) openBranch(ctx context.Context, config *branches.Config) (*Branch, error) {
	return OpenBranch(ctx, config, p.engine, p.Path, p)
}

func (p *Pool) OpenBranchByName(ctx context.Context, name string) (*Branch, error) {
	branchRef, err := p.LookupBranchByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return p.openBranch(ctx, branchRef)
}

// ResolveRevision returns the commit id for revision. revision can be either a
// commit ID in string form or a branch name.
func (p *Pool) ResolveRevision(ctx context.Context, revision string) (ksuid.KSUID, error) {
	id, err := dbid.ParseID(revision)
	if err != nil {
		branch, err := p.LookupBranchByName(ctx, revision)
		if err != nil {
			return ksuid.Nil, err
		}
		id = branch.Commit
	}
	return id, nil
}

func (p *Pool) BatchifyBranches(ctx context.Context, sctx *super.Context, recs []super.Value, m *sup.MarshalBSUPContext, f expr.Evaluator) ([]super.Value, error) {
	branches, err := p.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	for _, branchRef := range branches {
		meta := BranchMeta{p.Config, branchRef}
		rec, err := m.Marshal(&meta)
		if err != nil {
			return nil, err
		}
		if filter(sctx, rec, f) {
			recs = append(recs, rec)
		}
	}
	return recs, nil
}

func filter(sctx *super.Context, this super.Value, e expr.Evaluator) bool {
	if e == nil {
		return true
	}
	return expr.EvalBool(sctx, this, e).Ptr().AsBool()
}

type BranchTip struct {
	Name   string
	Commit ksuid.KSUID
}

func (p *Pool) BatchifyBranchTips(ctx context.Context, sctx *super.Context, f expr.Evaluator) ([]super.Value, error) {
	branches, err := p.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	m := sup.NewBSUPMarshalerWithContext(sctx)
	m.Decorate(sup.StylePackage)
	recs := make([]super.Value, 0, len(branches))
	for _, branchRef := range branches {
		rec, err := m.Marshal(&BranchTip{branchRef.Name, branchRef.Commit})
		if err != nil {
			return nil, err
		}
		if filter(sctx, rec, f) {
			recs = append(recs, rec)
		}
	}
	return recs, nil
}

// XXX this is inefficient but is only meant for interactive queries...?
func (p *Pool) ObjectExists(ctx context.Context, id ksuid.KSUID) (bool, error) {
	return p.engine.Exists(ctx, data.SequenceURI(p.DataPath, id))
}

func (p *Pool) Vacuum(ctx context.Context, commit ksuid.KSUID, dryrun bool) ([]ksuid.KSUID, error) {
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.GOMAXPROCS(0))
	ch := make(chan *data.Object)
	group.Go(func() error {
		defer close(ch)
		return p.commits.Vacuumable(ctx, commit, ch)
	})
	var vacuumed []ksuid.KSUID
	var mu sync.Mutex
	for o := range ch {
		o := o
		if dryrun {
			// For dryrun just check if the object exists and append existing
			// objects to list of results.
			group.Go(func() error {
				ok, err := p.engine.Exists(ctx, data.SequenceURI(p.DataPath, o.ID))
				if ok {
					mu.Lock()
					vacuumed = append(vacuumed, o.ID)
					mu.Unlock()
				}
				return err
			})
			continue
		}
		group.Go(func() error {
			err := p.engine.Delete(ctx, data.SequenceURI(p.DataPath, o.ID))
			if err == nil {
				mu.Lock()
				vacuumed = append(vacuumed, o.ID)
				mu.Unlock()
			}
			if errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
			return err
		})
		// Delete the seek index as well.
		group.Go(func() error {
			err := p.engine.Delete(ctx, data.SeekIndexURI(p.DataPath, o.ID))
			if errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return vacuumed, nil
}

func (p *Pool) Main(ctx context.Context) (BranchMeta, error) {
	branch, err := p.OpenBranchByName(ctx, "main")
	if err != nil {
		return BranchMeta{}, err
	}
	return BranchMeta{p.Config, branch.Config}, nil
}

func DataPath(poolPath *storage.URI) *storage.URI {
	return poolPath.JoinPath(DataTag)
}
