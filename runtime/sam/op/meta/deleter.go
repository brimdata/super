package meta

import (
	"errors"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zbuf"
	"github.com/segmentio/ksuid"
)

type Deleter struct {
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
	deletes     *sync.Map
}

func NewDeleter(rctx *runtime.Context, parent zbuf.Puller, pool *lake.Pool, filter zbuf.Filter, pruner expr.Evaluator, progress *zbuf.Progress, deletes *sync.Map) *Deleter {
	return &Deleter{
		parent:      parent,
		filter:      filter,
		pruner:      pruner,
		rctx:        rctx,
		pool:        pool,
		progress:    progress,
		unmarshaler: sup.NewZNGUnmarshaler(),
		deletes:     deletes,
	}
}

func (d *Deleter) Pull(done bool) (zbuf.Batch, error) {
	if d.done {
		return nil, d.err
	}
	if done {
		if d.scanner != nil {
			_, err := d.scanner.Pull(true)
			d.close(err)
			d.scanner = nil
		}
		return nil, d.err
	}
	for {
		if d.scanner == nil {
			scanner, err := d.nextDeletion()
			if scanner == nil || err != nil {
				d.close(err)
				return nil, err
			}
			d.scanner = scanner
		}
		if batch, err := d.scanner.Pull(false); err != nil {
			d.close(err)
			return nil, err
		} else if batch != nil {
			return batch, nil
		}
		d.scanner = nil
	}
}

func (d *Deleter) nextDeletion() (zbuf.Puller, error) {
	for {
		if d.parent == nil { //XXX
			return nil, nil
		}
		// Pull the next object to be scanned.  It must be an object
		// not a partition.
		batch, err := d.parent.Pull(false)
		if batch == nil || err != nil {
			return nil, err
		}
		vals := batch.Values()
		if len(vals) != 1 {
			// We currently support only one partition per batch.
			return nil, errors.New("internal error: meta.Deleter encountered multi-valued batch")
		}
		if hasDeletes, err := d.hasDeletes(vals[0]); err != nil {
			return nil, err
		} else if !hasDeletes {
			continue
		}
		// Use a no-op progress so stats are not inflated.
		var progress zbuf.Progress
		scanner, object, err := newScanner(d.rctx.Context, d.rctx.Zctx, d.pool, d.unmarshaler, d.pruner, d.filter, &progress, vals[0])
		if err != nil {
			return nil, err
		}
		d.deleteObject(object.ID)
		return scanner, nil
	}
}

func (d *Deleter) hasDeletes(val super.Value) (bool, error) {
	scanner, object, err := newScanner(d.rctx.Context, d.rctx.Zctx, d.pool, d.unmarshaler, d.pruner, d.filter, d.progress, val)
	if err != nil {
		return false, err
	}
	var count uint64
	for {
		batch, err := scanner.Pull(false)
		if err != nil {
			return false, err
		}
		if batch == nil {
			return count != object.Count, nil
		}
		count += uint64(len(batch.Values()))
	}
}

func (d *Deleter) close(err error) {
	d.err = err
	d.done = true
}

func (d *Deleter) deleteObject(id ksuid.KSUID) {
	d.deletes.Store(id, nil)
}
