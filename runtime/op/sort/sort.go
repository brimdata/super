package sort

import (
	"sync"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/field"
	"github.com/brimdata/zed/runtime/op"
	"github.com/brimdata/zed/runtime/op/spill"
	"github.com/brimdata/zed/zbuf"
)

// MemMaxBytes specifies the maximum amount of memory that each sort proc
// will consume.
var MemMaxBytes = 128 * 1024 * 1024

type Proc struct {
	pctx       *op.Context
	parent     zbuf.Puller
	order      order.Which
	nullsFirst bool

	fieldResolvers []expr.Evaluator
	once           sync.Once
	resultCh       chan op.Result
	comparator     *expr.Comparator
	ectx           expr.Context
	eof            bool
	sorter         expr.Sorter
}

func New(pctx *op.Context, parent zbuf.Puller, fields []expr.Evaluator, order order.Which, nullsFirst bool) (*Proc, error) {
	return &Proc{
		pctx:           pctx,
		parent:         parent,
		order:          order,
		nullsFirst:     nullsFirst,
		fieldResolvers: fields,
		resultCh:       make(chan op.Result),
	}, nil
}

func (p *Proc) Pull(done bool) (zbuf.Batch, error) {
	p.once.Do(func() { go p.run() })
	for {
		r, ok := <-p.resultCh
		if !ok {
			return nil, p.pctx.Err()
		}
		if !done || r.Batch == nil || r.Err != nil {
			return r.Batch, r.Err
		}
		r.Batch.Unref()
	}
}

func (p *Proc) run() {
	defer close(p.resultCh)
	var spiller *spill.MergeSort
	defer func() {
		if spiller != nil {
			spiller.Cleanup()
		}
	}()
	var nbytes int
	var out []zed.Value
	for {
		batch, err := p.parent.Pull(false)
		if err != nil {
			if ok := p.sendResult(nil, err); !ok {
				return
			}
			continue
		}
		if batch == nil {
			if spiller == nil {
				if len(out) > 0 {
					if ok := p.send(out); !ok {
						return
					}
				}
				if ok := p.sendResult(nil, nil); !ok {
					return
				}
				nbytes = 0
				out = nil
				continue
			}
			if len(out) > 0 {
				if err := spiller.Spill(p.pctx.Context, out); err != nil {
					if ok := p.sendResult(nil, err); !ok {
						return
					}
					spiller = nil
					nbytes = 0
					out = nil
					continue
				}
			}
			if ok := p.sendSpills(spiller); !ok {
				return
			}
			spiller.Cleanup()
			spiller = nil
			nbytes = 0
			out = nil
			continue
		}
		var delta int
		out, delta = p.append(out, batch)
		if p.comparator == nil && len(out) > 0 {
			p.setComparator(&out[0])
		}
		nbytes += delta
		if nbytes < MemMaxBytes {
			continue
		}
		if spiller == nil {
			spiller, err = spill.NewMergeSort(p.comparator)
			if err != nil {
				if ok := p.sendResult(nil, err); !ok {
					return
				}
				out = nil
				nbytes = 0
				continue
			}
		}
		if err := spiller.Spill(p.pctx.Context, out); err != nil {
			if ok := p.sendResult(nil, err); !ok {
				return
			}
		}
		out = nil
		nbytes = 0
	}
}

// send sorts vals in memory and sends the result downstream.
func (p *Proc) send(vals []zed.Value) bool {
	p.sorter.SortStable(vals, p.comparator)
	//XXX bug: we need upstream ectx. See #3367
	array := zbuf.NewArray(vals)
	return p.sendResult(array, nil)
}

func (p *Proc) sendSpills(spiller *spill.MergeSort) bool {
	puller := zbuf.NewPuller(spiller, 100)
	for {
		if err := p.pctx.Err(); err != nil {
			return false
		}
		// Reading from the spiller merges the spilt files.
		b, err := puller.Pull(false)
		if ok := p.sendResult(b, err); !ok {
			return false
		}
		if b == nil || err != nil {
			return true
		}
	}
}

func (p *Proc) sendResult(b zbuf.Batch, err error) bool {
	select {
	case p.resultCh <- op.Result{Batch: b, Err: err}:
		return true
	case <-p.pctx.Done():
		return false
	}
}

func (p *Proc) append(out []zed.Value, batch zbuf.Batch) ([]zed.Value, int) {
	var nbytes int
	vals := batch.Values()
	for i := range vals {
		val := &vals[i]
		nbytes += len(val.Bytes)
		// We're keeping records owned by batch so don't call Unref.
		out = append(out, *val)
	}
	return out, nbytes
}

func (p *Proc) setComparator(r *zed.Value) {
	resolvers := p.fieldResolvers
	if resolvers == nil {
		fld := GuessSortKey(r)
		resolver := expr.NewDottedExpr(p.pctx.Zctx, fld)
		resolvers = []expr.Evaluator{resolver}
	}
	reverse := p.order == order.Desc
	nullsMax := !p.nullsFirst
	if reverse {
		nullsMax = !nullsMax
	}
	p.comparator = expr.NewComparator(nullsMax, reverse, resolvers...).WithMissingAsNull()
}

func GuessSortKey(val *zed.Value) field.Path {
	recType := zed.TypeRecordOf(val.Type)
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
	return field.New("ts")
}

func firstMatchingField(typ *zed.TypeRecord, pred func(id int) bool) field.Path {
	for _, col := range typ.Columns {
		if pred(col.Type.ID()) {
			return field.New(col.Name)
		}
		if typ := zed.TypeRecordOf(col.Type); typ != nil {
			if f := firstMatchingField(typ, pred); f != nil {
				return append(field.New(col.Name), f...)
			}
		}
	}
	return nil
}
