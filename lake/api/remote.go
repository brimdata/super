package api

import (
	"context"
	"errors"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/api/client"
	"github.com/brimdata/super/api/queryio"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zngio"
	"github.com/segmentio/ksuid"
)

type remote struct {
	conn *client.Connection
}

var _ Interface = (*remote)(nil)

func NewRemoteLake(conn *client.Connection) Interface {
	return &remote{conn}
}

func (l *remote) Root() *lake.Root {
	return nil
}

func (r *remote) PoolID(ctx context.Context, poolName string) (ksuid.KSUID, error) {
	if id, err := lakeparse.ParseID(poolName); err == nil {
		if _, err := LookupPoolByID(ctx, r, id); err == nil {
			return id, nil
		}
	}
	config, err := LookupPoolByName(ctx, r, poolName)
	if err != nil {
		return ksuid.Nil, err
	}
	return config.ID, nil
}

func (r *remote) CommitObject(ctx context.Context, poolID ksuid.KSUID, branchName string) (ksuid.KSUID, error) {
	res, err := r.conn.BranchGet(ctx, poolID, branchName)
	return res.Commit, err
}

func (r *remote) CreatePool(ctx context.Context, name string, sortKeys order.SortKeys, seekStride int, thresh int64) (ksuid.KSUID, error) {
	res, err := r.conn.CreatePool(ctx, api.PoolPostRequest{
		Name: name,
		SortKeys: api.SortKeys{
			Order: sortKeys.Primary().Order,
			Keys:  field.List{sortKeys.Primary().Key},
		},
		SeekStride: seekStride,
		Thresh:     thresh,
	})
	if err != nil {
		return ksuid.Nil, err
	}
	return res.Pool.ID, err
}

func (r *remote) CreateBranch(ctx context.Context, poolID ksuid.KSUID, name string, at ksuid.KSUID) error {
	_, err := r.conn.CreateBranch(ctx, poolID, api.BranchPostRequest{
		Name:   name,
		Commit: at.String(),
	})
	return err
}

func (r *remote) RemoveBranch(ctx context.Context, poolID ksuid.KSUID, branchName string) error {
	return errors.New("TBD remote.RemoveBranch")
}

func (r *remote) MergeBranch(ctx context.Context, poolID ksuid.KSUID, childBranch, parentBranch string, message api.CommitMessage) (ksuid.KSUID, error) {
	res, err := r.conn.MergeBranch(ctx, poolID, childBranch, parentBranch, message)
	return res.Commit, err
}

func (r *remote) Compact(ctx context.Context, poolID ksuid.KSUID, branch string, objects []ksuid.KSUID, writeVectors bool, commit api.CommitMessage) (ksuid.KSUID, error) {
	res, err := r.conn.Compact(ctx, poolID, branch, objects, writeVectors, commit)
	return res.Commit, err
}

func (r *remote) RemovePool(ctx context.Context, pool ksuid.KSUID) error {
	return r.conn.RemovePool(ctx, pool)
}

func (r *remote) RenamePool(ctx context.Context, pool ksuid.KSUID, name string) error {
	if name == "" {
		return errors.New("no pool name provided")
	}
	return r.conn.RenamePool(ctx, pool, api.PoolPutRequest{Name: name})
}

func (r *remote) Load(ctx context.Context, _ *super.Context, poolID ksuid.KSUID, branchName string, reader zio.Reader, commit api.CommitMessage) (ksuid.KSUID, error) {
	pr, pw := io.Pipe()
	go func() {
		w := zngio.NewWriter(zio.NopCloser(pw))
		err := zio.CopyWithContext(ctx, w, reader)
		if err2 := w.Close(); err == nil {
			err = err2
		}
		pw.CloseWithError(err)
	}()
	res, err := r.conn.Load(ctx, poolID, branchName, api.MediaTypeZNG, pr, commit)
	return res.Commit, err
}

func (r *remote) Revert(ctx context.Context, poolID ksuid.KSUID, branchName string, commitID ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error) {
	res, err := r.conn.Revert(ctx, poolID, branchName, commitID, message)
	return res.Commit, err
}

func (r *remote) Query(ctx context.Context, head *lakeparse.Commitish, src string, srcfiles ...string) (zbuf.Scanner, error) {
	res, err := r.conn.Query(ctx, head, src, srcfiles...)
	if err != nil {
		return nil, err
	}
	return queryio.NewScanner(ctx, res.Body)
}

func (r *remote) Delete(ctx context.Context, poolID ksuid.KSUID, branchName string, tags []ksuid.KSUID, commit api.CommitMessage) (ksuid.KSUID, error) {
	res, err := r.conn.Delete(ctx, poolID, branchName, tags, commit)
	return res.Commit, err
}

func (r *remote) DeleteWhere(ctx context.Context, poolID ksuid.KSUID, branchName, src string, commit api.CommitMessage) (ksuid.KSUID, error) {
	res, err := r.conn.DeleteWhere(ctx, poolID, branchName, src, commit)
	return res.Commit, err
}

func (r *remote) AddVectors(ctx context.Context, pool, revision string, objects []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error) {
	res, err := r.conn.AddVectors(ctx, pool, revision, objects, message)
	return res.Commit, err
}

func (r *remote) DeleteVectors(ctx context.Context, pool, revision string, ids []ksuid.KSUID, message api.CommitMessage) (ksuid.KSUID, error) {
	res, err := r.conn.DeleteVectors(ctx, pool, revision, ids, message)
	return res.Commit, err
}

func (r *remote) Vacuum(ctx context.Context, pool, revision string, dryrun bool) ([]ksuid.KSUID, error) {
	res, err := r.conn.Vacuum(ctx, pool, revision, dryrun)
	return res.ObjectIDs, err
}
