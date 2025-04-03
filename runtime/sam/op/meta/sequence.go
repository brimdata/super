package meta

import (
	"context"
	"errors"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/data"
	"github.com/brimdata/super/lake/seekindex"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/merge"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio/zngio"
)

// SequenceScanner implements an op that pulls metadata partitions to scan
// from its parent and for each partition, scans the object.
type SequenceScanner struct {
	parent      zbuf.Puller
	scanner     zbuf.Puller
	filter      zbuf.Filter
	pruner      expr.Evaluator
	rctx        *runtime.Context
	pool        *lake.Pool
	progress    *zbuf.Progress
	unmarshaler *sup.UnmarshalZNGContext
	done        bool
	err         error
}

func NewSequenceScanner(rctx *runtime.Context, parent zbuf.Puller, pool *lake.Pool, filter zbuf.Filter, pruner expr.Evaluator, progress *zbuf.Progress) *SequenceScanner {
	return &SequenceScanner{
		rctx:        rctx,
		parent:      parent,
		filter:      filter,
		pruner:      pruner,
		pool:        pool,
		progress:    progress,
		unmarshaler: sup.NewZNGUnmarshaler(),
	}
}

func (s *SequenceScanner) Pull(done bool) (zbuf.Batch, error) {
	if s.done {
		return nil, s.err
	}
	if done {
		if s.scanner != nil {
			_, err := s.scanner.Pull(true)
			s.close(err)
			s.scanner = nil
		}
		return nil, s.err
	}
	for {
		if s.scanner == nil {
			batch, err := s.parent.Pull(false)
			if batch == nil || err != nil {
				s.close(err)
				return nil, err
			}
			vals := batch.Values()
			if len(vals) != 1 {
				// We currently support only one partition per batch.
				err := errors.New("system error: SequenceScanner encountered multi-valued batch")
				s.close(err)
				return nil, err
			}
			s.scanner, _, err = newScanner(s.rctx.Context, s.rctx.Zctx, s.pool, s.unmarshaler, s.pruner, s.filter, s.progress, vals[0])
			if err != nil {
				s.close(err)
				return nil, err
			}
		}
		batch, err := s.scanner.Pull(false)
		if err != nil {
			s.close(err)
			return nil, err
		}
		if batch != nil {
			return batch, nil
		}
		s.scanner = nil
	}
}

func (s *SequenceScanner) close(err error) {
	s.err = err
	s.done = true
}

type SearchScanner struct {
	filter   zbuf.Filter
	parent   Searcher
	pool     *lake.Pool
	progress *zbuf.Progress
	rctx     *runtime.Context
	scanner  zbuf.Puller
}

type Searcher interface {
	Pull(bool) (*data.Object, *vector.Bool, error)
}

func NewSearchScanner(rctx *runtime.Context, parent Searcher, pool *lake.Pool, filter zbuf.Filter, progress *zbuf.Progress) *SearchScanner {
	return &SearchScanner{
		filter:   filter,
		parent:   parent,
		pool:     pool,
		progress: progress,
		rctx:     rctx,
	}
}

func (s *SearchScanner) Pull(done bool) (zbuf.Batch, error) {
	if done {
		var err error
		if s.scanner != nil {
			_, err = s.scanner.Pull(true)
			s.scanner = nil
		}
		return nil, err
	}
	for {
		if s.scanner == nil {
			o, b, err := s.parent.Pull(done)
			if b == nil || err != nil {
				return nil, err
			}
			ranges, err := data.RangeFromBitVector(s.rctx.Context, s.pool.Storage(), s.pool.DataPath, o, b)
			if err != nil {
				return nil, err
			}
			if len(ranges) == 0 {
				continue
			}
			s.scanner, err = newObjectScanner(s.rctx.Context, s.rctx.Zctx, s.pool, o, ranges, s.filter, s.progress)
			if err != nil {
				return nil, err
			}
		}
		batch, err := s.scanner.Pull(false)
		if err != nil {
			return nil, err
		}
		if batch != nil {
			return batch, nil
		}
		s.scanner = nil
	}
}

func newScanner(ctx context.Context, zctx *super.Context, pool *lake.Pool, u *sup.UnmarshalZNGContext, pruner expr.Evaluator, filter zbuf.Filter, progress *zbuf.Progress, val super.Value) (zbuf.Puller, *data.Object, error) {
	named, ok := val.Type().(*super.TypeNamed)
	if !ok {
		return nil, nil, errors.New("system error: SequenceScanner encountered unnamed object")
	}
	var objects []*data.Object
	if named.Name == "data.Object" {
		var object data.Object
		if err := u.Unmarshal(val, &object); err != nil {
			return nil, nil, err
		}
		objects = []*data.Object{&object}
	} else {
		var part Partition
		if err := u.Unmarshal(val, &part); err != nil {
			return nil, nil, err
		}
		objects = part.Objects
	}
	scanner, err := newObjectsScanner(ctx, zctx, pool, objects, pruner, filter, progress)
	return scanner, objects[0], err
}

func newObjectsScanner(ctx context.Context, zctx *super.Context, pool *lake.Pool, objects []*data.Object, pruner expr.Evaluator, filter zbuf.Filter, progress *zbuf.Progress) (zbuf.Puller, error) {
	pullers := make([]zbuf.Puller, 0, len(objects))
	pullersDone := func() {
		for _, puller := range pullers {
			puller.Pull(true)
		}
	}
	for _, object := range objects {
		ranges, err := data.LookupSeekRange(ctx, pool.Storage(), pool.DataPath, object, pruner)
		if err != nil {
			return nil, err
		}
		s, err := newObjectScanner(ctx, zctx, pool, object, ranges, filter, progress)
		if err != nil {
			pullersDone()
			return nil, err
		}
		pullers = append(pullers, s)
	}
	if len(pullers) == 1 {
		return pullers[0], nil
	}
	return merge.New(ctx, pullers, lake.ImportComparator(zctx, pool).Compare, expr.Resetters{}), nil
}

func newObjectScanner(ctx context.Context, zctx *super.Context, pool *lake.Pool, object *data.Object, ranges []seekindex.Range, filter zbuf.Filter, progress *zbuf.Progress) (zbuf.Puller, error) {
	rc, err := object.NewReader(ctx, pool.Storage(), pool.DataPath, ranges)
	if err != nil {
		return nil, err
	}
	scanner, err := zngio.NewReader(zctx, rc).NewScanner(ctx, filter)
	if err != nil {
		rc.Close()
		return nil, err
	}
	return &statScanner{
		scanner:  scanner,
		closer:   rc,
		progress: progress,
	}, nil
}

type statScanner struct {
	scanner  zbuf.Scanner
	closer   io.Closer
	err      error
	progress *zbuf.Progress
}

func (s *statScanner) Pull(done bool) (zbuf.Batch, error) {
	if s.scanner == nil {
		return nil, s.err
	}
	batch, err := s.scanner.Pull(done)
	if batch == nil || err != nil {
		s.progress.Add(s.scanner.Progress())
		if err2 := s.closer.Close(); err == nil {
			err = err2
		}
		s.err = err
		s.scanner = nil
	}
	return batch, err
}
