package op

import (
	"errors"
	"fmt"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/data"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
)

type Scanner struct {
	parent     *objectPuller
	pruner     expr.Evaluator
	rctx       *runtime.Context
	pool       *lake.Pool
	once       sync.Once
	projection field.Projection
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
		parent:     newObjectPuller(parent),
		pruner:     pruner,
		pool:       pool,
		projection: field.NewProjection(paths),
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
		object, err := s.cache.Fetch(s.rctx.Context, meta.VectorURI(s.pool.DataPath), meta.ID)
		if err != nil {
			s.sendResult(nil, err)
			return
		}
		vec, err := object.Fetch(s.rctx.Sctx, s.projection)
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
	parent      zbuf.Puller
	unmarshaler *sup.UnmarshalBSUPContext
}

func newObjectPuller(parent zbuf.Puller) *objectPuller {
	return &objectPuller{
		parent:      parent,
		unmarshaler: sup.NewBSUPUnmarshaler(),
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
	named, ok := vals[0].Type().(*super.TypeNamed)
	if !ok {
		return nil, fmt.Errorf("system error: vam.objectPuller encountered unnamed object: %s", sup.String(vals[0]))
	}
	if named.Name != "data.Object" {
		return nil, fmt.Errorf("system error: vam.objectPuller encountered unnamed object: %q", named.Name)
	}
	var meta data.Object
	if err := p.unmarshaler.Unmarshal(vals[0], &meta); err != nil {
		return nil, fmt.Errorf("system error: vam.objectPuller could not unmarshal value: %q", sup.String(vals[0]))
	}
	return &meta, nil
}
