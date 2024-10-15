package sort

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/runtime/sam/op/spill"
	"github.com/brimdata/super/zbuf"
)

// MemMaxBytes specifies the maximum amount of memory that each sort proc
// will consume.
var MemMaxBytes = 128 * 1024 * 1024

type Op struct {
	rctx       *runtime.Context
	parent     zbuf.Puller
	nullsFirst bool
	resetter   expr.Resetter
	reverse    bool

	fieldResolvers []expr.SortEvaluator
	lastBatch      zbuf.Batch
	once           sync.Once
	resultCh       chan op.Result
	comparator     *expr.Comparator
}

func New(rctx *runtime.Context, parent zbuf.Puller, fields []expr.SortEvaluator, nullsFirst, reverse bool, resetter expr.Resetter) *Op {
	return &Op{
		rctx:           rctx,
		parent:         parent,
		nullsFirst:     nullsFirst,
		resetter:       resetter,
		reverse:        reverse,
		fieldResolvers: fields,
		resultCh:       make(chan op.Result),
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	o.once.Do(func() {
		// Block o.rctx.Cancel until p.run finishes its cleanup.
		o.rctx.WaitGroup.Add(1)
		go o.run()
	})
	for {
		r, ok := <-o.resultCh
		if !ok {
			return nil, o.rctx.Err()
		}
		if !done || r.Batch == nil || r.Err != nil {
			return r.Batch, r.Err
		}
		r.Batch.Unref()
	}
}

func (o *Op) run() {
	defer close(o.resultCh)
	var spiller *spill.MergeSort
	defer func() {
		if spiller != nil {
			spiller.Cleanup()
		}
		// Tell o.rctx.Cancel that we've finished our cleanup.
		o.rctx.WaitGroup.Done()
	}()
	var nbytes int
	var out []zed.Value
	for {
		batch, err := o.parent.Pull(false)
		if err != nil {
			if ok := o.sendResult(nil, err); !ok {
				return
			}
			continue
		}
		if batch == nil {
			if spiller == nil {
				if len(out) > 0 {
					if ok := o.send(out); !ok {
						return
					}
				}
				if ok := o.sendResult(nil, nil); !ok {
					return
				}
				nbytes = 0
				out = nil
				continue
			}
			if len(out) > 0 {
				if err := spiller.Spill(o.rctx.Context, out); err != nil {
					if ok := o.sendResult(nil, err); !ok {
						return
					}
					spiller = nil
					nbytes = 0
					out = nil
					continue
				}
			}
			if ok := o.sendSpills(spiller); !ok {
				return
			}
			spiller.Cleanup()
			spiller = nil
			nbytes = 0
			out = nil
			continue
		}
		// Safe because batch.Unref is never called.
		o.lastBatch = batch
		var delta int
		out, delta = o.append(out, batch)
		if o.comparator == nil && len(out) > 0 {
			o.setComparator(out[0])
		}
		nbytes += delta
		if nbytes < MemMaxBytes {
			continue
		}
		if spiller == nil {
			spiller, err = spill.NewMergeSort(o.comparator)
			if err != nil {
				if ok := o.sendResult(nil, err); !ok {
					return
				}
				out = nil
				nbytes = 0
				continue
			}
		}
		if err := spiller.Spill(o.rctx.Context, out); err != nil {
			if ok := o.sendResult(nil, err); !ok {
				return
			}
		}
		out = nil
		nbytes = 0
	}
}

// send sorts vals in memory and sends the result downstream.
func (o *Op) send(vals []zed.Value) bool {
	o.comparator.SortStable(vals)
	out := zbuf.NewBatch(o.lastBatch, vals)
	return o.sendResult(out, nil)
}

func (o *Op) sendSpills(spiller *spill.MergeSort) bool {
	puller := zbuf.NewPuller(spiller)
	for {
		if err := o.rctx.Err(); err != nil {
			return false
		}
		// Reading from the spiller merges the spilt files.
		b, err := puller.Pull(false)
		if ok := o.sendResult(b, err); !ok {
			return false
		}
		if b == nil || err != nil {
			return true
		}
	}
}

func (o *Op) sendResult(b zbuf.Batch, err error) bool {
	if b == nil && err == nil {
		// Reset stateful aggregation expressions on EOS.
		o.resetter.Reset()
	}
	select {
	case o.resultCh <- op.Result{Batch: b, Err: err}:
		return true
	case <-o.rctx.Done():
		return false
	}
}

func (o *Op) append(out []zed.Value, batch zbuf.Batch) ([]zed.Value, int) {
	var nbytes int
	vals := batch.Values()
	for i := range vals {
		val := &vals[i]
		nbytes += len(val.Bytes())
		// We're keeping records owned by batch so don't call Unref.
		out = append(out, *val)
	}
	return out, nbytes
}

func (o *Op) setComparator(r zed.Value) {
	resolvers := o.fieldResolvers
	if resolvers == nil {
		fld := GuessSortKey(r)
		resolver := expr.NewSortEvaluator(expr.NewDottedExpr(o.rctx.Zctx, fld), order.Asc)
		resolvers = []expr.SortEvaluator{resolver}
	}
	nullsMax := !o.nullsFirst
	if o.reverse {
		for k := range resolvers {
			resolvers[k].Order = !resolvers[k].Order
		}
	}
	if resolvers[0].Order == order.Desc {
		nullsMax = !nullsMax
	}
	o.comparator = expr.NewComparator(nullsMax, resolvers...).WithMissingAsNull()
}

func GuessSortKey(val zed.Value) field.Path {
	recType := zed.TypeRecordOf(val.Type())
	if recType == nil {
		// A nil field.Path is equivalent to "this".
		return nil
	}
	if f := firstMatchingField(recType, zed.IsInteger); f != nil {
		return f
	}
	if f := firstMatchingField(recType, zed.IsFloat); f != nil {
		return f
	}
	isNotTime := func(id int) bool { return id != zed.IDTime }
	if f := firstMatchingField(recType, isNotTime); f != nil {
		return f
	}
	return field.Path{"ts"}
}

func firstMatchingField(typ *zed.TypeRecord, pred func(id int) bool) field.Path {
	for _, f := range typ.Fields {
		if pred(f.Type.ID()) {
			return field.Path{f.Name}
		}
		if typ := zed.TypeRecordOf(f.Type); typ != nil {
			if ff := firstMatchingField(typ, pred); ff != nil {
				return append(field.Path{f.Name}, ff...)
			}
		}
	}
	return nil
}

func unrefBatches(batches []zbuf.Batch) {
	for _, b := range batches {
		b.Unref()
	}
}
