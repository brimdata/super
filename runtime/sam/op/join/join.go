package join

import (
	"context"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zio"
)

type Op struct {
	rctx        *runtime.Context
	style       string
	ctx         context.Context
	cancel      context.CancelFunc
	once        sync.Once
	left        *puller
	right       *zio.Peeker
	leftAlias   string
	rightAlias  string
	getLeftKey  expr.Evaluator
	getRightKey expr.Evaluator
	resetter    expr.Resetter
	compare     expr.CompareFn
	joinKey     *super.Value
	joinSet     []super.Value
	builder     zcode.Builder
}

func New(rctx *runtime.Context, style string, left, right zbuf.Puller, leftKey, rightKey expr.Evaluator,
	leftAlias, rightAlias string, leftDir, rightDir order.Direction, resetter expr.Resetter) *Op {
	if style == "right" {
		leftKey, rightKey = rightKey, leftKey
		left, right = right, left
		leftDir, rightDir = rightDir, leftDir
	}
	var o order.Which
	switch {
	case leftDir != order.Unknown:
		o = leftDir == order.Down
	case rightDir != order.Unknown:
		o = rightDir == order.Down
	}
	// Add sorts if needed.
	if !leftDir.HasOrder(o) {
		s := expr.NewSortExpr(leftKey, o, order.NullsLast)
		left = sort.New(rctx, left, []expr.SortExpr{s}, false, resetter)
	}
	if !rightDir.HasOrder(o) {
		s := expr.NewSortExpr(rightKey, o, order.NullsLast)
		right = sort.New(rctx, right, []expr.SortExpr{s}, false, resetter)
	}
	ctx, cancel := context.WithCancel(rctx.Context)
	return &Op{
		rctx:        rctx,
		style:       style,
		ctx:         ctx,
		cancel:      cancel,
		leftAlias:   leftAlias,
		rightAlias:  rightAlias,
		getLeftKey:  leftKey,
		getRightKey: rightKey,
		left:        newPuller(left, ctx),
		right:       zio.NewPeeker(newPuller(right, ctx)),
		resetter:    resetter,
		compare:     expr.NewValueCompareFn(o, o.NullsMax(true)),
	}
}

// Pull implements the merge logic for returning data from the upstreams.
func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	// XXX see issue #3437 regarding done protocol.
	o.once.Do(func() {
		go o.left.run()
		go o.right.Reader.(*puller).run()
	})
	var out []super.Value
	// See #3366
	ectx := expr.NewContext()
	for {
		leftRec, err := o.left.Read()
		if err != nil {
			return nil, err
		}
		if leftRec == nil {
			if len(out) == 0 {
				o.resetter.Reset()
				return nil, nil
			}
			//XXX See issue #3427.
			return zbuf.NewArray(out), nil
		}
		key := o.getLeftKey.Eval(ectx, *leftRec)
		if key.IsMissing() {
			// If the left key isn't present (which is not a thing
			// in a sql join), then drop the record and return only
			// left records that can eval the key expression.
			continue
		}
		rightRecs, err := o.getJoinSet(key)
		if err != nil {
			return nil, err
		}
		if rightRecs == nil {
			// Nothing to add to the left join.
			// Accumulate this record for an outer join.
			if o.style != "inner" {
				out = append(out, o.wrap(leftRec, nil))
			}
			continue
		}
		if o.style == "anti" {
			continue
		}
		// For every record on the right with a key matching
		// this left record, generate a joined record.
		// XXX This loop could be more efficient if we had CutAppend
		// and built the record in a re-usable buffer, then allocated
		// a right-sized output buffer for the record body and copied
		// the two inputs into the output buffer.  Even better, these
		// output buffers could come from a large buffer that implements
		// Batch and lives in a pool so the downstream user can
		// release the batch with and bypass GC.
		for _, rightRec := range rightRecs {
			out = append(out, o.wrap(leftRec, rightRec.Ptr()))
		}
	}
}

func (o *Op) getJoinSet(leftKey super.Value) ([]super.Value, error) {
	if o.joinKey != nil && o.compare(leftKey, *o.joinKey) == 0 {
		return o.joinSet, nil
	}
	// See #3366
	ectx := expr.NewContext()
	for {
		rec, err := o.right.Peek()
		if err != nil || rec == nil {
			return nil, err
		}
		rightKey := o.getRightKey.Eval(ectx, *rec)
		if rightKey.IsMissing() {
			o.right.Read()
			continue
		}
		cmp := o.compare(leftKey, rightKey)
		if cmp == 0 {
			// Copy leftKey.Bytes since it might get reused.
			if o.joinKey == nil {
				o.joinKey = leftKey.Copy().Ptr()
			} else {
				o.joinKey.CopyFrom(leftKey)
			}
			o.joinSet, err = o.readJoinSet(o.joinKey)
			return o.joinSet, err
		}
		if cmp < 0 {
			// If the left key is smaller than the next eligible
			// join key, then there is nothing to join for this
			// record.
			return nil, nil
		}
		// Discard the peeked-at record and keep looking for
		// a righthand key that either matches or exceeds the
		// lefthand key.
		o.right.Read()
	}
}

// fillJoinSet is called when a join key has been found that matches
// the current lefthand key.  It returns the all the subsequent records
// from the righthand stream that match this key.
func (o *Op) readJoinSet(joinKey *super.Value) ([]super.Value, error) {
	var recs []super.Value
	// See #3366
	ectx := expr.NewContext()
	for {
		rec, err := o.right.Peek()
		if err != nil {
			return nil, err
		}
		if rec == nil {
			return recs, nil
		}
		key := o.getRightKey.Eval(ectx, *rec)
		if key.IsMissing() {
			o.right.Read()
			continue
		}
		if o.compare(key, *joinKey) != 0 {
			return recs, nil
		}
		recs = append(recs, rec.Copy())
		o.right.Read()
	}
}

func (o *Op) wrap(l, r *super.Value) super.Value {
	if o.style == "right" {
		l, r = r, l
	}
	o.builder.Reset()
	var fields []super.Field
	if l != nil {
		left := l.Under()
		fields = append(fields, super.Field{Name: o.leftAlias, Type: left.Type()})
		o.builder.Append(left.Bytes())
	}
	if r != nil {
		right := r.Under()
		fields = append(fields, super.Field{Name: o.rightAlias, Type: right.Type()})
		o.builder.Append(right.Bytes())

	}
	return super.NewValue(o.rctx.Sctx.MustLookupTypeRecord(fields), o.builder.Bytes())
}
