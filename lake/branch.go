package lake

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/lake/branches"
	"github.com/brimdata/zed/lake/commits"
	"github.com/brimdata/zed/lake/data"
	"github.com/brimdata/zed/lake/index"
	"github.com/brimdata/zed/lake/journal"
	"github.com/brimdata/zed/lakeparse"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/runtime"
	"github.com/brimdata/zed/runtime/op"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
)

const (
	maxCommitRetries  = 10
	maxMessageObjects = 10
)

var (
	ErrCommitFailed      = fmt.Errorf("exceeded max update attempts (%d) to branch tip: commit failed", maxCommitRetries)
	ErrInvalidCommitMeta = errors.New("cannot parse ZSON string")
)

type Branch struct {
	branches.Config
	pool   *Pool
	engine storage.Engine
	//base   commits.View
}

func OpenBranch(ctx context.Context, config *branches.Config, engine storage.Engine, poolPath *storage.URI, pool *Pool) (*Branch, error) {
	return &Branch{
		Config: *config,
		pool:   pool,
		engine: engine,
	}, nil
}

func (b *Branch) Load(ctx context.Context, zctx *zed.Context, r zio.Reader, author, message, meta string) (ksuid.KSUID, error) {
	w, err := NewWriter(ctx, zctx, b.pool)
	if err != nil {
		return ksuid.Nil, err
	}
	err = zio.CopyWithContext(ctx, w, r)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return ksuid.Nil, err
	}
	objects := w.Objects()
	if len(objects) == 0 {
		return ksuid.Nil, commits.ErrEmptyTransaction
	}
	if message == "" {
		message = loadMessage(objects)
	}
	appMeta, err := loadMeta(zctx, meta)
	if err != nil {
		return ksuid.Nil, err
	}
	// The load operation has only added new objects so we know its
	// safe to merge at the tip and there can be no conflicts
	// with other concurrent writers (except for updating the branch pointer
	// which is handled by Branch.commit)
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		return commits.NewAddsObject(parent.Commit, retries, author, message, *appMeta, objects), nil
	})
}

func loadMessage(objects []data.Object) string {
	var b strings.Builder
	plural := "s"
	if len(objects) == 1 {
		plural = ""
	}
	b.WriteString(fmt.Sprintf("loaded %d data object%s\n\n", len(objects), plural))
	for k, o := range objects {
		b.WriteString("  ")
		b.WriteString(o.String())
		b.WriteByte('\n')
		if k >= maxMessageObjects {
			b.WriteString("  ...\n")
			break
		}
	}
	return b.String()
}

func loadMeta(zctx *zed.Context, meta string) (*zed.Value, error) {
	if meta == "" {
		return zed.Null, nil
	}
	zv, err := zson.ParseValue(zed.NewContext(), meta)
	if err != nil {
		return zctx.Missing(), fmt.Errorf("%w %s: %v", ErrInvalidCommitMeta, zv, err)
	}
	return zv, nil
}

func (b *Branch) Delete(ctx context.Context, ids []ksuid.KSUID, author, message string) (ksuid.KSUID, error) {
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		snap, err := b.pool.commits.Snapshot(ctx, parent.Commit)
		if err != nil {
			return nil, err
		}
		var objects []*data.Object
		for _, id := range ids {
			o, err := snap.Lookup(id)
			if err != nil {
				return nil, err
			}
			objects = append(objects, o)
		}
		if message == "" {
			var b strings.Builder
			fmt.Fprintf(&b, "deleted %d data object%s\n\n", len(objects), plural(objects))
			printObjects(&b, objects, maxMessageObjects)
			message = b.String()
		}
		return commits.NewDeletesObject(parent.Commit, retries, author, message, ids), nil
	})
}

func (b *Branch) DeleteWhere(ctx context.Context, c runtime.Compiler, program ast.Op, author, message, meta string) (ksuid.KSUID, error) {
	zctx := zed.NewContext()
	appMeta, err := loadMeta(zctx, meta)
	if err != nil {
		return ksuid.Nil, err
	}
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		pctx := op.NewContext(ctx, zctx, nil)
		defer pctx.Cancel()
		// XXX It would be great to not do this since and just pass the snapshot
		// into c.NewLakeDeleteQuery since we have to load the snapshot later
		// anyways. Unfortunately there's quite a few layers of plumbing needed
		// to get this working in compiler.
		commitish := &lakeparse.Commitish{
			Pool:   b.pool.Name,
			Branch: parent.Commit.String(),
		}
		query, err := c.NewLakeDeleteQuery(pctx, program, commitish)
		if err != nil {
			return nil, err
		}
		defer query.Pull(true)
		w, err := NewWriter(ctx, zctx, b.pool)
		if err != nil {
			return nil, err
		}
		err = zio.CopyWithContext(ctx, w, query.AsReader())
		if closeErr := w.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}
		base, err := b.pool.commits.Snapshot(ctx, parent.Commit)
		if err != nil {
			return nil, err
		}
		deleted := query.DeletionSet()
		if len(deleted) == 0 {
			return nil, commits.ErrEmptyTransaction
		}
		patch := commits.NewPatch(base)
		for _, oid := range deleted {
			patch.DeleteObject(oid)
		}
		for _, o := range w.Objects() {
			obj := o
			patch.AddDataObject(&obj)
		}
		if message == "" {
			var deletedObjs []*data.Object
			for _, id := range deleted {
				o, _ := base.Lookup(id)
				deletedObjs = append(deletedObjs, o)
			}
			var added []*data.Object
			for _, o := range w.Objects() {
				added = append(added, &o)
			}
			message = deleteWhereMessage(deletedObjs, added)
		}
		return patch.NewCommitObject(parent.Commit, retries, author, message, *appMeta), nil
	})
}

func deleteWhereMessage(deleted, added []*data.Object) string {
	var b strings.Builder
	fmt.Fprintf(&b, "deleted %d data object%s\n\n", len(deleted), plural(deleted))
	printObjects(&b, deleted, maxMessageObjects)
	if len(added) > 0 {
		fmt.Fprintf(&b, "\nadded %d data object%s\n\n", len(added), plural(added))
		printObjects(&b, added, maxMessageObjects-len(deleted))
	}
	return b.String()
}

func printObjects(b *strings.Builder, objects []*data.Object, maxMessageObjects int) {
	for k, o := range objects {
		fmt.Fprintf(b, "  %s\n", o)
		if k >= maxMessageObjects {
			b.WriteString("  ...\n")
		}
	}
}

func plural[S ~[]E, E any](s S) string {
	if len(s) == 1 {
		return ""
	}
	return "s"
}

func (b *Branch) Revert(ctx context.Context, commit ksuid.KSUID, author, message string) (ksuid.KSUID, error) {
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		patch, err := b.pool.commits.PatchOfCommit(ctx, commit)
		if err != nil {
			return nil, fmt.Errorf("commit not found: %s", commit)
		}
		tip, err := b.pool.commits.Snapshot(ctx, parent.Commit)
		if err != nil {
			return nil, err
		}
		if message == "" {
			message = fmt.Sprintf("reverted commit %s", commit)
		}
		object, err := patch.Revert(tip, ksuid.New(), parent.Commit, retries, author, message)
		if err != nil {
			return nil, err
		}
		return object, nil
	})
}

func (b *Branch) CommitCompact(ctx context.Context, src, rollup []*data.Object, author, message, meta string) (ksuid.KSUID, error) {
	if len(rollup) < 1 {
		return ksuid.Nil, errors.New("compact: one or more rollup objects required")
	}
	zctx := zed.NewContext()
	appMeta, err := loadMeta(zctx, meta)
	if err != nil {
		return ksuid.Nil, err
	}
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		base, err := b.pool.commits.Snapshot(ctx, parent.Commit)
		if err != nil {
			return nil, err
		}
		patch := commits.NewPatch(base)
		for _, o := range rollup {
			if err := patch.AddDataObject(o); err != nil {
				return nil, err
			}
		}
		for _, o := range src {
			if err := patch.DeleteObject(o.ID); err != nil {
				return nil, err
			}
		}
		if message == "" {
			var b strings.Builder
			fmt.Fprintf(&b, "compacted %d object%s\n\n", len(src), plural(src))
			printObjects(&b, src, maxMessageObjects)
			fmt.Fprintf(&b, "\ncreated %d object%s\n\n", len(rollup), plural(rollup))
			printObjects(&b, rollup, maxMessageObjects-len(src))
			message = b.String()
		}
		commit := patch.NewCommitObject(parent.Commit, retries, author, message, *appMeta)
		return commit, nil
	})
}

func (b *Branch) mergeInto(ctx context.Context, parent *Branch, author, message string) (ksuid.KSUID, error) {
	if b == parent {
		return ksuid.Nil, errors.New("cannot merge branch into itself")
	}
	return parent.commit(ctx, func(head *branches.Config, retries int) (*commits.Object, error) {
		return b.buildMergeObject(ctx, head, retries, author, message, parent.Name)
	})
	//XXX we should follow parent commit with a child rebase... do this
	// next... we want to fast forward the child to any pending commits
	// that happened on the child branch while we were merging into the parent.
	// and rebase the child branch to point at the parent where we grafted on
	// it's ok if new commits are arriving past the parent graft on point...
}

func (b *Branch) buildMergeObject(ctx context.Context, parent *branches.Config, retries int, author, message, parentName string) (*commits.Object, error) {
	childPath, err := b.pool.commits.Path(ctx, b.Commit)
	if err != nil {
		return nil, err
	}
	parentPath, err := b.pool.commits.Path(ctx, parent.Commit)
	if err != nil {
		return nil, err
	}
	baseID := commonAncestor(parentPath, childPath)
	if baseID == ksuid.Nil {
		//XXX this shouldn't happen because because all of the branches
		// should live in a single tree.
		//XXX hmm, except if you branch main when it is empty...?
		// we shoudl detect this and not allow it...?
		return nil, errors.New("system error: cannot locate common ancestor for branch merge")
	}
	// Compute the snapshot of the common ancestor then compute patches
	// along each branch and make sure the two patches do not have a
	// delete conflict.  For now, this is the only kind of merge update
	// conflict we detect.
	base, err := b.pool.commits.Snapshot(ctx, baseID)
	if err != nil {
		return nil, err
	}
	childPatch, err := b.pool.commits.PatchOfPath(ctx, base, baseID, b.Commit)
	if err != nil {
		return nil, err
	}
	parentPatch, err := b.pool.commits.PatchOfPath(ctx, base, baseID, parent.Commit)
	if err != nil {
		return nil, err
	}
	if message == "" {
		message = fmt.Sprintf("merged %q into %q", b.Name, parent.Name)
	}
	// Now compute the diff between the parent patch and the child patch so that
	// the diff patch will reflect the changes from the child into the parent.
	// Diff() will also check for delete conflicts.
	diff, err := commits.Diff(parentPatch, childPatch)
	if err != nil {
		return nil, fmt.Errorf("error merging %q into %q: %w", b.Name, parent.Name, err)
	}
	return diff.NewCommitObject(parent.Commit, retries, author, message, *zed.Null), nil
}

func commonAncestor(a, b []ksuid.KSUID) ksuid.KSUID {
	m := make(map[ksuid.KSUID]struct{})
	for _, id := range a {
		m[id] = struct{}{}
	}
	for _, id := range b {
		if _, ok := m[id]; ok {
			return id
		}
	}
	return ksuid.Nil
}

type constructor func(parent *branches.Config, retries int) (*commits.Object, error)

func (b *Branch) commit(ctx context.Context, create constructor) (ksuid.KSUID, error) {
	// A commit must append new state to the tip of the branch while simultaneously
	// upating the branch pointer in a trasactionally consistent fashion.
	// For example, if we compute a commit object based on a certain tip commit,
	// then commit that object after another writer commits in between,
	// the commit object may be inconsistent against the intervening commit.
	//
	// We do this update optimistically and ensure this consistency with
	// a loop that builds the commit object based on the presumed parent,
	// then moves the branch pointer to the new commit but, using a constraint,
	// only succeeds when the presumed parent is atomically consistent
	// with the branch update.  If the contraint, fails will loop a number
	// of times till it succeeds, or we give up.
	for retries := 0; retries < maxCommitRetries; retries++ {
		config, err := b.pool.branches.LookupByName(ctx, b.Name)
		if err != nil {
			return ksuid.Nil, err
		}
		object, err := create(config, retries)
		if err != nil {
			return ksuid.Nil, err
		}
		if err := b.pool.commits.Put(ctx, object); err != nil {
			return ksuid.Nil, fmt.Errorf("branch %q failed to write commit object: %w", b.Name, err)
		}
		// Set the branch pointer to point to this commit object
		// and stash the current commit (that will become the parent)
		// in a local for the constraint check closure.
		parent := config.Commit
		config.Commit = object.Commit
		parentCheck := func(e journal.Entry) bool {
			if entry, ok := e.(*branches.Config); ok {
				return entry.Commit == parent
			}
			return false
		}
		if err := b.pool.branches.Update(ctx, config, parentCheck); err != nil {
			if err == journal.ErrConstraint {
				// Parent check failed so try again.
				if err := b.pool.commits.Remove(ctx, object); err != nil {
					return ksuid.Nil, err
				}
				continue
			}
			return ksuid.Nil, err
		}
		return object.Commit, nil
	}
	return ksuid.Nil, fmt.Errorf("branch %q: %w", b.Name, ErrCommitFailed)
}

func (b *Branch) LookupTags(ctx context.Context, tags []ksuid.KSUID) ([]ksuid.KSUID, error) {
	var ids []ksuid.KSUID
	for _, tag := range tags {
		ok, err := b.pool.ObjectExists(ctx, tag)
		if err != nil {
			return nil, err
		}
		if ok {
			ids = append(ids, tag)
			continue
		}
		patch, err := b.pool.commits.PatchOfCommit(ctx, tag)
		if err != nil {
			continue
		}
		ids = append(ids, patch.DataObjects()...)
	}
	return ids, nil
}

func (b *Branch) Pool() *Pool {
	return b.pool
}

func (b *Branch) ApplyIndexRules(ctx context.Context, c runtime.Compiler, rules []index.Rule, ids []ksuid.KSUID) (ksuid.KSUID, error) {
	idxrefs := make([]*index.Object, 0, len(rules)*len(ids))
	for _, id := range ids {
		//XXX make issue for this.
		// This could be easily parallized with errgroup.
		refs, err := b.indexObject(ctx, c, rules, id)
		if err != nil {
			return ksuid.Nil, err
		}
		idxrefs = append(idxrefs, refs...)
	}
	author := "indexer"
	message := indexMessage(rules)
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		return commits.NewAddIndexesObject(parent.Commit, author, message, retries, idxrefs), nil
	})
}

func (b *Branch) UpdateIndex(ctx context.Context, c runtime.Compiler, rules []index.Rule) (ksuid.KSUID, error) {
	snap, err := b.pool.commits.Snapshot(ctx, b.Commit)
	if err != nil {
		return ksuid.Nil, err
	}
	var objects []*index.Object
	for id, rules := range snap.Unindexed(rules) {
		o, err := b.indexObject(ctx, c, rules, id)
		if err != nil {
			return ksuid.Nil, err
		}
		objects = append(objects, o...)
	}
	if len(objects) == 0 {
		return ksuid.Nil, commits.ErrEmptyTransaction
	}
	const author = "indexer"
	message := indexMessage(rules)
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		return commits.NewAddIndexesObject(parent.Commit, author, message, retries, objects), nil
	})
}

func indexMessage(rules []index.Rule) string {
	skip := make(map[string]struct{})
	var names []string
	for _, r := range rules {
		name := r.RuleName()
		if _, ok := skip[name]; !ok {
			skip[name] = struct{}{}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "index rules applied: " + strings.Join(names, ",")
}

func (b *Branch) indexObject(ctx context.Context, c runtime.Compiler, rules []index.Rule, id ksuid.KSUID) ([]*index.Object, error) {
	r, err := b.engine.Get(ctx, data.SequenceURI(b.pool.DataPath, id))
	if err != nil {
		return nil, err
	}
	reader := zngio.NewReader(zed.NewContext(), r)
	defer reader.Close()
	w, err := index.NewCombiner(ctx, c, b.engine, b.pool.IndexPath, rules, id)
	if err != nil {
		r.Close()
		return nil, err
	}
	err = zio.CopyWithContext(ctx, w, reader)
	if err != nil {
		w.Abort()
	} else {
		err = w.Close()
	}
	if rerr := r.Close(); err == nil {
		err = rerr
	}
	return w.References(), err
}

func (b *Branch) AddVectors(ctx context.Context, ids []ksuid.KSUID, author, message string) (ksuid.KSUID, error) {
	if message == "" {
		message = vectorMessage("add", ids)
	}
	// XXX We should add some parallelism here to stream the next file while
	// the CPU is chugging away on the current file.  See issue #4015.
	for _, id := range ids {
		if err := data.CreateVector(ctx, b.pool.engine, b.pool.DataPath, id); err != nil {
			return ksuid.Nil, err
		}
	}
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		snap, err := b.pool.commits.Snapshot(ctx, parent.Commit)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if !snap.Exists(id) {
				return nil, fmt.Errorf("non-existent object %s: vector add operation aborted", id)
			}
			if snap.HasVector(id) {
				return nil, fmt.Errorf("vector exists for %s: vector add operation aborted", id)
			}
		}
		return commits.NewAddVectorsObject(parent.Commit, author, message, ids, retries), nil
	})
}

func (b *Branch) DeleteVectors(ctx context.Context, ids []ksuid.KSUID, author, message string) (ksuid.KSUID, error) {
	if message == "" {
		message = vectorMessage("delete", ids)
	}
	return b.commit(ctx, func(parent *branches.Config, retries int) (*commits.Object, error) {
		snap, err := b.pool.commits.Snapshot(ctx, parent.Commit)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if !snap.Exists(id) {
				return nil, fmt.Errorf("non-existent object %s: vector delete operation aborted", id)
			}
			if !snap.HasVector(id) {
				return nil, fmt.Errorf("vector %s does not exist: vector delete operation aborted", id)
			}
		}
		return commits.NewDeleteVectorsObject(parent.Commit, author, message, ids, retries), nil
	})
}

func vectorMessage(kind string, ids []ksuid.KSUID) string {
	var b strings.Builder
	b.WriteString("vector ")
	b.WriteString(kind)
	b.WriteString("\n\n")
	for _, id := range ids {
		b.WriteString("    ")
		b.WriteString(id.String())
		b.WriteByte('\n')
	}
	return b.String()
}
