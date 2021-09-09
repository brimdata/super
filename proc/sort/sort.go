package sort

import (
	"fmt"
	"sync"

	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/field"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/proc"
	"github.com/brimdata/zed/proc/spill"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zng"
)

// MemMaxBytes specifies the maximum amount of memory that each sort proc
// will consume.
var MemMaxBytes = 128 * 1024 * 1024

type Proc struct {
	pctx       *proc.Context
	parent     proc.Interface
	order      order.Which
	nullsFirst bool

	fieldResolvers     []expr.Evaluator
	once               sync.Once
	resultCh           chan proc.Result
	compareFn          expr.CompareFn
	unseenFieldTracker *unseenFieldTracker
}

func New(pctx *proc.Context, parent proc.Interface, fields []expr.Evaluator, order order.Which, nullsFirst bool) (*Proc, error) {
	return &Proc{
		pctx:               pctx,
		parent:             parent,
		order:              order,
		nullsFirst:         nullsFirst,
		fieldResolvers:     fields,
		resultCh:           make(chan proc.Result),
		unseenFieldTracker: newUnseenFieldTracker(fields),
	}, nil
}

func (p *Proc) Pull() (zbuf.Batch, error) {
	p.once.Do(func() { go p.sortLoop() })
	if r, ok := <-p.resultCh; ok {
		return r.Batch, r.Err
	}
	return nil, p.pctx.Err()
}

func (p *Proc) Done() {
	p.parent.Done()
}

func (p *Proc) sortLoop() {
	defer close(p.resultCh)
	firstRunRecs, eof, err := p.recordsForOneRun()
	if err != nil || len(firstRunRecs) == 0 {
		p.sendResult(nil, err)
		return
	}
	p.setCompareFn(firstRunRecs[0])
	if eof {
		// Just one run so do an in-memory sort.
		p.warnAboutUnseenFields()
		expr.SortStable(firstRunRecs, p.compareFn)
		array := zbuf.Array(firstRunRecs)
		p.sendResult(array, nil)
		return
	}
	// Multiple runs so do an external merge sort.
	runManager, err := p.createRuns(firstRunRecs)
	if err != nil {
		p.sendResult(nil, err)
		return
	}
	defer runManager.Cleanup()
	p.warnAboutUnseenFields()
	puller := zbuf.NewPuller(runManager, 100)
	for p.pctx.Err() == nil {
		// Reading from runManager merges the runs.
		b, err := puller.Pull()
		p.sendResult(b, err)
		if b == nil || err != nil {
			return
		}
	}
}

func (p *Proc) sendResult(b zbuf.Batch, err error) {
	select {
	case p.resultCh <- proc.Result{Batch: b, Err: err}:
	case <-p.pctx.Done():
	}
}

func (p *Proc) recordsForOneRun() ([]*zng.Record, bool, error) {
	var nbytes int
	var recs []*zng.Record
	for {
		batch, err := p.parent.Pull()
		if err != nil {
			return nil, false, err
		}
		if batch == nil {
			return recs, true, nil
		}
		l := batch.Length()
		for i := 0; i < l; i++ {
			rec := batch.Index(i)
			p.unseenFieldTracker.update(rec)
			nbytes += len(rec.Bytes)
			// We're keeping records owned by batch so don't call Unref.
			recs = append(recs, rec)
		}
		if nbytes >= MemMaxBytes {
			return recs, false, nil
		}
	}
}

func (p *Proc) createRuns(firstRunRecs []*zng.Record) (*spill.MergeSort, error) {
	rm, err := spill.NewMergeSort(p.compareFn)
	if err != nil {
		return nil, err
	}
	if err := rm.Spill(p.pctx.Context, firstRunRecs); err != nil {
		rm.Cleanup()
		return nil, err
	}
	for {
		recs, eof, err := p.recordsForOneRun()
		if err != nil {
			rm.Cleanup()
			return nil, err
		}
		if recs != nil {
			if err := rm.Spill(p.pctx.Context, recs); err != nil {
				rm.Cleanup()
				return nil, err
			}
		}
		if eof {
			return rm, nil
		}
	}
}

func (p *Proc) warnAboutUnseenFields() {
	for _, f := range p.unseenFieldTracker.unseen() {
		name, _ := expr.DotExprToString(f)
		p.pctx.Warnings <- fmt.Sprintf("Sort field %s not present in input", name)
	}
}

func (p *Proc) setCompareFn(r *zng.Record) {
	resolvers := p.fieldResolvers
	if resolvers == nil {
		fld := GuessSortKey(r)
		resolver := expr.NewDotExpr(fld)
		resolvers = []expr.Evaluator{resolver}
	}
	nullsMax := !p.nullsFirst
	if p.order == order.Desc {
		nullsMax = !nullsMax
	}
	compareFn := expr.NewCompareFn(nullsMax, resolvers...)
	if p.order == order.Desc {
		p.compareFn = func(a, b *zng.Record) int { return compareFn(b, a) }
	} else {
		p.compareFn = compareFn
	}
}

func GuessSortKey(rec *zng.Record) field.Path {
	typ := zng.TypeRecordOf(rec.Type)
	if f := firstMatchingField(typ, zng.IsInteger); f != nil {
		return f
	}
	if f := firstMatchingField(typ, zng.IsFloat); f != nil {
		return f
	}
	isNotTime := func(id int) bool { return id != zng.IDTime }
	if f := firstMatchingField(typ, isNotTime); f != nil {
		return f
	}
	return field.New("ts")
}

func firstMatchingField(typ *zng.TypeRecord, pred func(id int) bool) field.Path {
	for _, col := range typ.Columns {
		if pred(col.Type.ID()) {
			return field.New(col.Name)
		}
		if typ := zng.TypeRecordOf(col.Type); typ != nil {
			if f := firstMatchingField(typ, pred); f != nil {
				return append(field.New(col.Name), f...)
			}
		}
	}
	return nil
}
