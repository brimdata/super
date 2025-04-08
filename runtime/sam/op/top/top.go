package top

import (
	"container/heap"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/zbuf"
)

const defaultTopLimit = 100

// Top is similar to op.Sort with a view key differences:
// - It only sorts in descending order.
// - It utilizes a MaxHeap, immediately discarding records that are not in
// the top N of the sort.
// - It has a hidden option (FlushEvery) to sort and emit on every batch.
type Op struct {
	parent     zbuf.Puller
	sctx       *super.Context
	limit      int
	fields     []expr.Evaluator
	flushEvery bool
	resetter   expr.Resetter
	records    *expr.RecordSlice
	compare    expr.CompareFn
}

func New(sctx *super.Context, parent zbuf.Puller, limit int, fields []expr.Evaluator, flushEvery bool, resetter expr.Resetter) *Op {
	if limit == 0 {
		limit = defaultTopLimit
	}
	return &Op{
		parent:     parent,
		limit:      limit,
		fields:     fields,
		flushEvery: flushEvery,
		resetter:   resetter,
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	for {
		batch, err := o.parent.Pull(done)
		if err != nil {
			return nil, err
		}
		if batch == nil {
			defer o.resetter.Reset()
			return o.sorted(), nil
		}
		vals := batch.Values()
		for i := range vals {
			o.consume(vals[i])
		}
		batch.Unref()
		if o.flushEvery {
			return o.sorted(), nil
		}
	}
}

func (o *Op) consume(rec super.Value) {
	if o.fields == nil {
		fld := sort.GuessSortKey(rec)
		accessor := expr.NewDottedExpr(o.sctx, fld)
		o.fields = []expr.Evaluator{accessor}
	}
	if o.records == nil {
		o.compare = expr.NewCompareFn(false, o.fields...)
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
	if o.records == nil {
		return nil
	}
	out := make([]super.Value, o.records.Len())
	for i := o.records.Len() - 1; i >= 0; i-- {
		out[i] = heap.Pop(o.records).(super.Value)
	}
	// clear records
	o.records = nil
	return zbuf.NewArray(out)
}
