package op

import (
	"errors"
	"fmt"
	"sync"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/lake/data"
	"github.com/brimdata/zed/pkg/field"
	"github.com/brimdata/zed/runtime"
	"github.com/brimdata/zed/runtime/sam/expr"
	"github.com/brimdata/zed/runtime/vcache"
	"github.com/brimdata/zed/vector"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zson"
)

type Scanner struct {
	parent     *objectPuller
	pruner     expr.Evaluator
	rctx       *runtime.Context
	pool       *lake.Pool
	once       sync.Once
	projection vcache.Path
	cache      *vcache.Cache
	progress   *zbuf.Progress
	resultCh   chan result
	doneCh     chan struct{}
}

var _ vector.Puller = (*Scanner)(nil)

func NewScanner(rctx *runtime.Context, cache *vcache.Cache, parent zbuf.Puller, pool *lake.Pool, paths []field.Path, pruner expr.Evaluator, progress *zbuf.Progress) *Scanner {
	return &Scanner{
		cache:      cache,
		rctx:       rctx,
		parent:     newObjectPuller(rctx.Zctx, parent),
		pruner:     pruner,
		pool:       pool,
		projection: vcache.NewProjection(paths),
		progress:   progress,
		doneCh:     make(chan struct{}),
		resultCh:   make(chan result),
	}
}

// XXX we need vector scannerstats and means to update them here.

// XXX change this to pull/load vector by each type within an object and
// return an object containing the overall projection, which might be a record
// or could just be a single vector.  the downstream operator has to be
// configured to expect it, e.g., project x:=a.b,y:=a.b.c (like cut but in vspace)
// this would be Record{x:(proj a.b),y:(proj:a.b.c)} so the elements would be
// single fields.  For each object/type that matches the projection we would make
// a Record vec and let GC reclaim them.  Note if a col is missing, it's a constant
// vector of error("missing").

func (s *Scanner) Pull(done bool) (vector.Any, error) {
	s.once.Do(func() { go s.run() })
	if done {
		select {
		case s.doneCh <- struct{}{}:
			return nil, nil
		case <-s.rctx.Done():
			return nil, s.rctx.Err()
		}
	}
	if r, ok := <-s.resultCh; ok {
		return r.vector, r.err
	}
	return nil, s.rctx.Err()
}

func (s *Scanner) run() {
	for {
		meta, err := s.parent.Pull(false)
		if meta == nil {
			s.sendResult(nil, err)
			return
		}
		object, err := s.cache.Fetch(s.rctx.Context, s.rctx.Zctx, meta.VectorURI(s.pool.DataPath), meta.ID)
		if err != nil {
			s.sendResult(nil, err)
			return
		}
		vec, err := object.Fetch(s.rctx.Zctx, s.projection)
		s.sendResult(vec, err)
		if err != nil {
			return
		}
	}
}

func (s *Scanner) sendResult(vec vector.Any, err error) (bool, bool) {
	select {
	case s.resultCh <- result{vec, err}:
		return false, true
	case <-s.doneCh:
		_, pullErr := s.parent.Pull(true)
		if err == nil {
			err = pullErr
		}
		if err != nil {
			select {
			case s.resultCh <- result{err: err}:
				return true, false
			case <-s.rctx.Done():
				return false, false
			}
		}
		return true, true
	case <-s.rctx.Done():
		return false, false
	}
}

type result struct {
	vector vector.Any
	err    error //XXX go err vs vector.Any err?
}

type objectPuller struct {
	parent zbuf.Puller
	zctx   *zed.Context
}

func newObjectPuller(zctx *zed.Context, parent zbuf.Puller) *objectPuller {
	return &objectPuller{
		parent: parent,
		zctx:   zctx,
	}
}

func (p *objectPuller) Pull(done bool) (*data.Object, error) {
	batch, err := p.parent.Pull(false)
	if batch == nil || err != nil {
		return nil, err
	}
	defer batch.Unref()
	vals := batch.Values()
	if len(vals) != 1 {
		// We require exactly one data object per pull.
		return nil, errors.New("system error: vam.objectPuller encountered multi-valued batch")
	}
	named, ok := vals[0].Type().(*zed.TypeNamed)
	if !ok {
		return nil, fmt.Errorf("system error: vam.objectPuller encountered unnamed object: %s", zson.String(vals[0]))
	}
	if named.Name != "data.Object" {
		return nil, fmt.Errorf("system error: vam.objectPuller encountered unnamed object: %q", named.Name)
	}
	arena := zed.NewArena()
	var meta data.Object
	if err := zson.UnmarshalZNG(p.zctx, arena, vals[0], &meta); err != nil {
		return nil, fmt.Errorf("system error: vam.objectPuller could not unmarshal value %q: %w", zson.String(vals[0]), err)
	}
	meta.Arena = arena
	return &meta, nil
}
