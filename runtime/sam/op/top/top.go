package top

import (
	"container/heap"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/zbuf"
)

// Top produces the first N values that sort would produce with the same arguments.
type Op struct {
	sctx       *super.Context
	parent     zbuf.Puller
	limit      int
	exprs      []expr.SortExpr
	nullsFirst bool
	reverse    bool
	resetter   expr.Resetter

	eos     bool
	records *expr.RecordSlice
	compare expr.CompareFn
}

// New returns an operator that produces the first limit
func New(sctx *super.Context, parent zbuf.Puller, limit int, exprs []expr.SortExpr, nullsFirst, reverse bool, resetter expr.Resetter) *Op {
	return &Op{
		sctx:       sctx,
		parent:     parent,
		limit:      limit,
		exprs:      exprs,
		nullsFirst: nullsFirst,
		reverse:    reverse,
		resetter:   resetter,
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	if o.eos {
		o.eos = false
		return nil, nil
	}
	for {
		batch, err := o.parent.Pull(done)
		if err != nil {
			return nil, err
		}
		if batch == nil {
			if o.records == nil {
				return nil, nil
			}
			o.eos = true
			defer o.resetter.Reset()
			return o.sorted(), nil
		}
		vals := batch.Values()
		for i := range vals {
			o.consume(vals[i])
		}
		batch.Unref()
	}
}

func (o *Op) consume(rec super.Value) {
	if o.records == nil {
		if o.compare == nil {
			// Package heap implements a min-heap.  Invert o.nullsFirst and o.reverse to get a max-heap.
			o.compare = sort.NewComparator(o.sctx, o.exprs, !o.nullsFirst, !o.reverse, rec).Compare
		}
		o.records = expr.NewRecordSlice(o.compare)
		heap.Init(o.records)
	}
	if o.records.Len() < o.limit || o.compare(o.records.Index(0), rec) < 0 {
		heap.Push(o.records, rec.Copy())
	}
	if o.records.Len() > o.limit {
		heap.Pop(o.records)
	}
}

func (o *Op) sorted() zbuf.Batch {
	out := make([]super.Value, o.records.Len())
	for i := o.records.Len() - 1; i >= 0; i-- {
		out[i] = heap.Pop(o.records).(super.Value)
	}
	// clear records
	o.records = nil
	return zbuf.NewArray(out)
}
