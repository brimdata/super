package api

import (
	"context"
	"errors"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/lake/index"
	"github.com/brimdata/zed/lakeparse"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/runtime"
	"github.com/brimdata/zed/runtime/exec"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
)

type local struct {
	root     *lake.Root
	compiler runtime.Compiler
	engine   storage.Engine
}

var _ Interface = (*local)(nil)

func OpenLocalLake(ctx context.Context, logger *zap.Logger, lakePath string) (Interface, error) {
	uri, err := storage.ParseURI(lakePath)
	if err != nil {
		return nil, err
	}
	engine := storage.NewLocalEngine()
	root, err := lake.Open(ctx, engine, logger, uri)
	if err != nil {
		return nil, err
	}
	return &local{
		root:     root,
		compiler: compiler.NewLakeCompiler(root),
		engine:   engine,
	}, nil
}

func CreateLocalLake(ctx context.Context, logger *zap.Logger, lakePath string) (Interface, error) {
	uri, err := storage.ParseURI(lakePath)
	if err != nil {
		return nil, err
	}
	engine := storage.NewLocalEngine()
	root, err := lake.Create(ctx, engine, logger, uri)
	if err != nil {
		return nil, err
	}
	return &local{
		root:   root,
		engine: engine,
	}, nil
}

func (l *local) Root() *lake.Root {
	return l.root
}

func (l *local) CreatePool(ctx context.Context, name string, sortKey order.SortKey, seekStride int, thresh int64) (ksuid.KSUID, error) {
	if name == "" {
		return ksuid.Nil, errors.New("no pool name provided")
	}
	pool, err := l.root.CreatePool(ctx, name, sortKey, seekStride, thresh)
	if err != nil {
		return ksuid.Nil, err
	}
	return pool.ID, nil
}

func (l *local) RemovePool(ctx context.Context, id ksuid.KSUID) error {
	return l.root.RemovePool(ctx, id)

}

func (l *local) RenamePool(ctx context.Context, id ksuid.KSUID, name string) error {
	if name == "" {
		return errors.New("no pool name provided")
	}
	return l.root.RenamePool(ctx, id, name)
}

func (l *local) CreateBranch(ctx context.Context, poolID ksuid.KSUID, name string, parent ksuid.KSUID) error {
	_, err := l.root.CreateBranch(ctx, poolID, name, parent)
	return err
}

func (l *local) RemoveBranch(ctx context.Context, poolID ksuid.KSUID, branchName string) error {
	return l.root.RemoveBranch(ctx, poolID, branchName)
}

func (l *local) MergeBranch(ctx context.Context, poolID ksuid.KSUID, childBranch, parentBranch string, message api.CommitMessage) (ksuid.KSUID, error) {
	return l.root.MergeBranch(ctx, poolID, childBranch, parentBranch, message.Author, message.Body)
}

func (l *local) Compact(ctx context.Context, poolID ksuid.KSUID, branchName string, objects []ksuid.KSUID, commit api.CommitMessage) (ksuid.KSUID, error) {
	pool, err := l.root.OpenPool(ctx, poolID)
	if err != nil {
		return ksuid.Nil, err
	}
	return exec.Compact(ctx, l.root, pool, branchName, objects, commit.Author, commit.Body, commit.Meta)
}

func (l *local) AddIndexRules(ctx context.Context, rules []index.Rule) error {
	return l.root.AddIndexRules(ctx, rules)
}

func (l *local) DeleteIndexRules(ctx context.Context, ids []ksuid.KSUID) ([]index.Rule, error) {
	return l.root.DeleteIndexRules(ctx, ids)
}

func (l *local) Query(ctx context.Context, head *lakeparse.Commitish, src string, srcfiles ...string) (zio.ReadCloser, error) {
	q, err := l.QueryWithControl(ctx, head, src, srcfiles...)
	if err != nil {
		return nil, err
	}
	return zio.NewReadCloser(zbuf.NoControl(q), q), nil
}

func (l *local) QueryWithControl(ctx context.Context, head *lakeparse.Commitish, src string, srcfiles ...string) (zbuf.ProgressReadCloser, error) {
	flowgraph, err := l.compiler.Parse(src, srcfiles...)
	if err != nil {
		return nil, err
	}
	q, err := runtime.CompileLakeQuery(ctx, zed.NewContext(), l.compiler, flowgraph, head, nil)
	if err != nil {
		return nil, err
	}
	return q.AsProgressReadCloser(), nil
}

func (l *local) PoolID(ctx context.Context, poolName string) (ksuid.KSUID, error) {
	if poolName == "" {
		return ksuid.Nil, errors.New("no pool name provided")
	}
	return l.root.PoolID(ctx, poolName)
}

func (l *local) CommitObject(ctx context.Context, poolID ksuid.KSUID, branchName string) (ksuid.KSUID, error) {
	return l.root.CommitObject(ctx, poolID, branchName)
}

func (l *local) lookupBranch(ctx context.Context, poolID ksuid.KSUID, branchName string) (*lake.Pool, *lake.Branch, error) {
	pool, err := l.root.OpenPool(ctx, poolID)
	if err != nil {
		return nil, nil, err
	}
	branch, err := pool.OpenBranchByName(ctx, branchName)
	if err != nil {
		return nil, nil, err
	}
	return pool, branch, nil
}

func (l *local) Load(ctx context.Context, ztcx *zed.Context, poolID ksuid.KSUID, branchName string, r zio.Reader, message api.CommitMessage) (ksuid.KSUID, error) {
	_, branch, err := l.lookupBranch(ctx, poolID, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	return branch.Load(ctx, ztcx, r, message.Author, message.Body, message.Meta)
}

func (l *local) Delete(ctx context.Context, poolID ksuid.KSUID, branchName string, ids []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error) {
	_, branch, err := l.lookupBranch(ctx, poolID, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	commitID, err := branch.Delete(ctx, ids, message.Author, message.Body)
	if err != nil {
		return ksuid.Nil, err
	}
	return commitID, nil
}

func (l *local) DeleteWhere(ctx context.Context, poolID ksuid.KSUID, branchName, src string, commit api.CommitMessage) (ksuid.KSUID, error) {
	op, err := l.compiler.Parse(src)
	if err != nil {
		return ksuid.Nil, err
	}
	_, branch, err := l.lookupBranch(ctx, poolID, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	return branch.DeleteWhere(ctx, l.compiler, op, commit.Author, commit.Body, commit.Meta)
}

func (l *local) Revert(ctx context.Context, poolID ksuid.KSUID, branchName string, commitID ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error) {
	return l.root.Revert(ctx, poolID, branchName, commitID, message.Author, message.Body)
}

func (l *local) ApplyIndexRules(ctx context.Context, ruleRefs []string, poolID ksuid.KSUID, branchName string, inTags []ksuid.KSUID) (ksuid.KSUID, error) {
	_, branch, err := l.lookupBranch(ctx, poolID, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	tags, err := branch.LookupTags(ctx, inTags)
	if err != nil {
		return ksuid.Nil, err
	}
	rules, err := l.root.LookupIndexRules(ctx, lakeparse.FormatIDs(ruleRefs)...)
	if err != nil {
		return ksuid.Nil, err
	}
	commit, err := branch.ApplyIndexRules(ctx, l.compiler, rules, tags)
	if err != nil {
		return ksuid.Nil, err
	}
	return commit, nil
}

func (l *local) UpdateIndex(ctx context.Context, ruleRefs []string, poolID ksuid.KSUID, branchName string) (ksuid.KSUID, error) {
	_, branch, err := l.lookupBranch(ctx, poolID, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	var rules []index.Rule
	if len(ruleRefs) == 0 {
		rules, err = l.root.AllIndexRules(ctx)
	} else {
		rules, err = l.root.LookupIndexRules(ctx, lakeparse.FormatIDs(ruleRefs)...)
	}
	if err != nil {
		return ksuid.Nil, err
	}
	return branch.UpdateIndex(ctx, l.compiler, rules)
}

func (l *local) AddVectors(ctx context.Context, poolID ksuid.KSUID, branchName string, ids []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error) {
	_, branch, err := l.lookupBranch(ctx, poolID, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	return branch.AddVectors(ctx, ids, message.Author, message.Body)
}

func (l *local) DeleteVectors(ctx context.Context, poolID ksuid.KSUID, branchName string, ids []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error) {
	_, branch, err := l.lookupBranch(ctx, poolID, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	return branch.DeleteVectors(ctx, ids, message.Author, message.Body)
}
