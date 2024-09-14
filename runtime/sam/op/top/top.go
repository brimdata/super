package top

import (
	"container/heap"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/runtime/sam/expr"
	"github.com/brimdata/zed/runtime/sam/op/sort"
	"github.com/brimdata/zed/zbuf"
)

const defaultTopLimit = 100

// Top is similar to op.Sort with a view key differences:
// - It only sorts in descending order.
// - It utilizes a MaxHeap, immediately discarding records that are not in
// the top N of the sort.
// - It has a hidden option (FlushEvery) to sort and emit on every batch.
type Op struct {
	parent     zbuf.Puller
	zctx       *zed.Context
	limit      int
	fields     []expr.Evaluator
	flushEvery bool
	resetter   expr.Resetter
	records    *expr.RecordSlice
	compare    expr.CompareFn
}

func New(zctx *zed.Context, parent zbuf.Puller, limit int, fields []expr.Evaluator, flushEvery bool, resetter expr.Resetter) *Op {
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

func (o *Op) consume(rec zed.Value) {
	if o.fields == nil {
		fld := sort.GuessSortKey(rec)
		accessor := expr.NewDottedExpr(o.zctx, fld)
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
	out := make([]zed.Value, o.records.Len())
	for i := o.records.Len() - 1; i >= 0; i-- {
		out[i] = heap.Pop(o.records).(zed.Value)
	}
	// clear records
	o.records = nil
	return zbuf.NewArray(out)
}
