package lake

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/lake/branches"
	"github.com/brimdata/zed/lake/commits"
	"github.com/brimdata/zed/lake/data"
	"github.com/brimdata/zed/lake/pools"
	"github.com/brimdata/zed/lakeparse"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/runtime/sam/expr"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
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

func (p *Pool) OpenCommitLog(ctx context.Context, zctx *zed.Context, commit ksuid.KSUID) zio.Reader {
	return p.commits.OpenCommitLog(ctx, zctx, commit, ksuid.Nil)
}

func (p *Pool) OpenCommitLogAsZNG(ctx context.Context, zctx *zed.Context, commit ksuid.KSUID) (*zngio.Reader, error) {
	return p.commits.OpenAsZNG(ctx, zctx, commit, ksuid.Nil)
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
	id, err := lakeparse.ParseID(revision)
	if err != nil {
		branch, err := p.LookupBranchByName(ctx, revision)
		if err != nil {
			return ksuid.Nil, err
		}
		id = branch.Commit
	}
	return id, nil
}

func (p *Pool) BatchifyBranches(ctx context.Context, zctx *zed.Context, recs []zed.Value, m *zson.MarshalZNGContext, f expr.Evaluator) ([]zed.Value, error) {
	branches, err := p.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	ectx := expr.NewContext()
	for _, branchRef := range branches {
		meta := BranchMeta{p.Config, branchRef}
		rec, err := m.Marshal(&meta)
		if err != nil {
			return nil, err
		}
		if filter(zctx, ectx, rec, f) {
			recs = append(recs, rec)
		}
	}
	return recs, nil
}

func filter(zctx *zed.Context, ectx expr.Context, this zed.Value, e expr.Evaluator) bool {
	if e == nil {
		return true
	}
	val, ok := expr.EvalBool(zctx, ectx, this, e)
	return ok && val.Bool()
}

type BranchTip struct {
	Name   string
	Commit ksuid.KSUID
}

func (p *Pool) BatchifyBranchTips(ctx context.Context, zctx *zed.Context, f expr.Evaluator) ([]zed.Value, error) {
	branches, err := p.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	m := zson.NewZNGMarshalerWithContext(zctx)
	m.Decorate(zson.StylePackage)
	recs := make([]zed.Value, 0, len(branches))
	ectx := expr.NewContext()
	for _, branchRef := range branches {
		rec, err := m.Marshal(&BranchTip{branchRef.Name, branchRef.Commit})
		if err != nil {
			return nil, err
		}
		if filter(zctx, ectx, rec, f) {
			recs = append(recs, rec)
		}
	}
	return recs, nil
}

// XXX this is inefficient but is only meant for interactive queries...?
func (p *Pool) ObjectExists(ctx context.Context, id ksuid.KSUID) (bool, error) {
	return p.engine.Exists(ctx, data.SequenceURI(p.DataPath, id))
}

func (p *Pool) Vacate(ctx context.Context, ts nano.Ts, dryrun bool) ([]ksuid.KSUID, error) {
	if !dryrun {
		if err := p.vacateBranchStore(ctx, ts, dryrun); err != nil {
			return nil, err
		}
	}
	return p.vacateCommits(ctx, ts, dryrun)
}

func (p *Pool) vacateCommits(ctx context.Context, ts nano.Ts, dryrun bool) ([]ksuid.KSUID, error) {
	main, err := p.Main(ctx)
	if err != nil {
		return nil, err
	}
	commit, err := p.commits.FindNearestToTs(ctx, main.Branch.Commit, ts)
	if err != nil {
		return nil, err
	}
	branches, err := p.branches.All(ctx)
	if err != nil {
		return nil, err
	}
	var conflicts []string
	for _, b := range branches {
		// Find any branches whose history does not include commit and if any
		// are found fail.
		path, err := p.commits.Path(ctx, b.Commit)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(path, commit) {
			conflicts = append(conflicts, b.Name)
		}
	}
	if len(conflicts) > 0 {
		slices.Sort(conflicts)
		v := "branch does not include commit nearest timestamp"
		if len(conflicts) > 1 {
			v = "branches do not include commit nearest timestamp"
		}
		return nil, fmt.Errorf("cannot vacate at selected time: %s in the deletion path: %s", v, strings.Join(conflicts, ", "))
	}
	if dryrun {
		path, err := p.commits.Path(ctx, commit)
		if err != nil {
			return nil, err
		}
		return path[1:], nil
	}
	return p.commits.SetBase(ctx, commit)
}

func (p *Pool) vacateBranchStore(ctx context.Context, ts nano.Ts, dryrun bool) error {
	var werr error
	at, err := p.branches.EntryWhere(ctx, func(config *branches.Config) bool {
		if config.Commit.IsNil() {
			return false
		}
		var c *commits.Commit
		_, c, werr = p.commits.GetBytes(ctx, config.Commit)
		return werr != nil || ts >= c.Date
	})
	if err = errors.Join(err, werr); err != nil {
		return err
	}
	return p.branches.TruncateHistory(ctx, at)
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
