package top

import (
	"container/heap"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/sbuf"
)

// Top produces the first N values that sort would produce with the same arguments.
type Op struct {
	sctx         *super.Context
	parent       sbuf.Puller
	limit        int
	exprs        []expr.SortExpr
	guessReverse bool

	eos     bool
	records *expr.RecordSlice
	compare expr.CompareFn
}

// New returns an operator that produces the first limit
func New(sctx *super.Context, parent sbuf.Puller, limit int, exprs []expr.SortExpr, guessReverse bool) *Op {
	return &Op{
		sctx:         sctx,
		parent:       parent,
		limit:        limit,
		exprs:        exprs,
		guessReverse: guessReverse,
	}
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
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
			comparator := sort.NewComparator(o.sctx, o.exprs, rec, o.guessReverse)
			// package heap implements a min-heap.  Invert the comparison result to get a max-heap.
			o.compare = func(a, b super.Value) int { return comparator.Compare(a, b) * -1 }
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

func (o *Op) sorted() sbuf.Batch {
	out := make([]super.Value, o.records.Len())
	for i := o.records.Len() - 1; i >= 0; i-- {
		out[i] = heap.Pop(o.records).(super.Value)
	}
	// clear records
	o.records = nil
	return sbuf.NewArray(out)
}
