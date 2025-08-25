package dbmanage

import (
	"context"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/db/api"
	"github.com/brimdata/super/db/data"
	"github.com/brimdata/super/db/pools"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/extent"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sup"
	"github.com/segmentio/ksuid"
)

func scan(ctx context.Context, it *objectIterator, pool *pools.Config, runCh chan<- []ksuid.KSUID, vecCh chan<- ksuid.KSUID) error {
	send := func(r *runBuilder) error {
		switch len(r.objects) {
		case 0: // do nothing
		case 1:
			if !r.objects[0].Vector {
				select {
				case vecCh <- r.objects[0].ID:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		default:
			select {
			case runCh <- r.objectIDs():
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	run := newRunBuilder()
	for {
		o, err := it.next()
		if o == nil {
			return send(run)
		}
		if err != nil {
			return err
		}
		if run.overlaps(o.Min, o.Max) || run.size+o.Size < pool.Threshold {
			run.add(o)
			continue
		}
		if err := send(run); err != nil {
			return err
		}
		run.reset()
		run.add(o)
	}
}

const iteratorQuery = `
from %q@%q:objects
| left join (from %q@%q:vectors) on id
| values {...left, vector: has(right)}
| sort min
`

type objectIterator struct {
	reader      sio.Reader
	puller      sbuf.Puller
	unmarshaler *sup.UnmarshalBSUPContext
}

func newObjectIterator(ctx context.Context, db api.Interface, head *dbid.Commitish) (*objectIterator, error) {
	query := fmt.Sprintf(iteratorQuery, head.Pool, head.Branch, head.Pool, head.Branch)
	q, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return &objectIterator{
		reader:      sbuf.PullerReader(q),
		puller:      q,
		unmarshaler: sup.NewBSUPUnmarshaler(),
	}, nil
}

func (r *objectIterator) next() (*object, error) {
	val, err := r.reader.Read()
	if val == nil || err != nil {
		return nil, err
	}
	var o object
	// XXX Embedded structs currently not supported in zed marshal so unmarshal
	// embedded object struct separately.
	if err := r.unmarshaler.Unmarshal(*val, &o.Object); err != nil {
		return nil, err
	}
	if err := r.unmarshaler.Unmarshal(*val, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *objectIterator) close() error {
	_, err := r.puller.Pull(true)
	return err
}

type object struct {
	data.Object
	Vector bool `super:"vector"`
}

type runBuilder struct {
	span    extent.Span
	cmp     expr.CompareFn
	objects []*object
	size    int64
}

func newRunBuilder() *runBuilder {
	return &runBuilder{cmp: expr.NewValueCompareFn(order.Asc, order.NullsLast)}
}

func (r *runBuilder) overlaps(first, last super.Value) bool {
	if r.span == nil {
		return false
	}
	return r.span.Overlaps(first, last)
}

func (r *runBuilder) add(o *object) {
	r.objects = append(r.objects, o)
	r.size += o.Size
	if r.span == nil {
		r.span = extent.NewGeneric(o.Min, o.Max, r.cmp)
		return
	}
	r.span.Extend(o.Min)
	r.span.Extend(o.Max)
}

func (r *runBuilder) objectIDs() []ksuid.KSUID {
	var ids []ksuid.KSUID
	for _, o := range r.objects {
		ids = append(ids, o.ID)
	}
	return ids
}

func (r *runBuilder) reset() {
	r.span = nil
	r.objects = r.objects[:0]
	r.size = 0
}
