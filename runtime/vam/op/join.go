package op

import (
	"context"
	"encoding/binary"
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/join"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type Join struct {
	rctx     *runtime.Context
	anti     bool
	inner    bool
	left     vector.Puller
	right    vector.Puller
	leftKey  expr.Evaluator
	rightKey expr.Evaluator

	cutter   *samexpr.Cutter
	splicer  *join.RecordSplicer
	hashJoin *hashJoin
}

func NewJoin(rctx *runtime.Context, anti, inner bool, left, right vector.Puller, leftKey, rightKey expr.Evaluator, lhs []*samexpr.Lval, rhs []samexpr.Evaluator) *Join {
	return &Join{
		rctx:     rctx,
		anti:     anti,
		inner:    inner,
		left:     left,
		right:    right,
		leftKey:  leftKey,
		rightKey: rightKey,
		cutter:   samexpr.NewCutter(rctx.Sctx, lhs, rhs),
		splicer:  join.NewRecordSplicer(rctx.Sctx),
	}
}

func (j *Join) Pull(done bool) (vector.Any, error) {
	if done {
		_, err := j.left.Pull(true)
		if err == nil {
			_, err = j.right.Pull(true)
		}
		j.hashJoin = nil
		return nil, err
	}
	if j.hashJoin == nil {
		if err := j.tableInit(); err != nil {
			return nil, err
		}
	}
	vec, err := j.hashJoin.Pull()
	if vec == nil || err != nil {
		j.hashJoin = nil
	}
	return vec, err
}

func (j *Join) tableInit() error {
	// Read from both left and right parent and find the shortest parent to
	// create the table from.
	var left, right *bufPuller
	done := new(atomic.Bool)
	group, ctx := errgroup.WithContext(j.rctx)
	group.Go(func() error {
		var err error
		left, err = readAllRace(ctx, done, j.left)
		return err
	})
	group.Go(func() error {
		var err error
		right, err = readAllRace(ctx, done, j.right)
		return err
	})
	if err := group.Wait(); err != nil {
		return err
	}
	leftKey, rightKey := j.leftKey, j.rightKey
	if !right.EOS {
		left, right = right, left
		leftKey, rightKey = rightKey, leftKey
	}
	table := map[string][]super.Value{}
	var keyBuilder, valBuilder zcode.Builder
	for {
		vec, _ := right.Pull(false)
		if vec == nil {
			break
		}
		rightKeyVec := j.rightKey.Eval(vec)
		for i := range vec.Len() {
			keyBuilder.Truncate()
			keyVal := vectorValue(&keyBuilder, rightKeyVec, i)
			if keyVal.IsMissing() {
				continue
			}
			key := hashKey(keyVal)
			valBuilder.Reset()
			table[key] = append(table[key], vectorValue(&valBuilder, vec, i))
		}
	}
	j.hashJoin = &hashJoin{
		anti:     j.anti,
		inner:    j.inner,
		left:     left,
		table:    table,
		leftKey:  leftKey,
		rightKey: rightKey,
		cutter:   j.cutter,
		splicer:  j.splicer,
	}
	return nil
}

func readAllRace(ctx context.Context, done *atomic.Bool, parent vector.Puller) (*bufPuller, error) {
	b := &bufPuller{puller: parent}
	for ctx.Err() == nil || done.Load() {
		vec, err := parent.Pull(false)
		if vec == nil || err != nil {
			done.Store(true)
			b.EOS = true
			return b, err
		}
		b.vecs = append(b.vecs, vec)
	}
	return b, nil
}

type hashJoin struct {
	anti     bool
	inner    bool
	left     vector.Puller
	table    map[string][]super.Value
	leftKey  expr.Evaluator
	rightKey expr.Evaluator
	cutter   *samexpr.Cutter
	splicer  *join.RecordSplicer
}

func (j *hashJoin) Pull() (vector.Any, error) {
	for {
		vec, err := j.left.Pull(false)
		if vec == nil || err != nil {
			return nil, err
		}
		leftKeyVec := j.leftKey.Eval(vec)
		var keyBuilder, valBuilder zcode.Builder
		b := vector.NewDynamicBuilder()
		for i := range vec.Len() {
			keyBuilder.Truncate()
			keyVal := vectorValue(&keyBuilder, leftKeyVec, i)
			if keyVal.IsMissing() {
				continue
			}
			key := hashKey(keyVal)
			valBuilder.Truncate()
			leftVal := vectorValue(&valBuilder, vec, i)
			rightVals, ok := j.table[key]
			if !ok {
				if !j.inner {
					b.Write(leftVal)
				}
				continue
			}
			if j.anti {
				continue
			}
			for _, rightVal := range rightVals {
				cutVal := j.cutter.Eval(nil, rightVal)
				val, err := j.splicer.Splice(leftVal, cutVal)
				if err != nil {
					return nil, err
				}
				b.Write(val)
			}
		}
		out := b.Build()
		if out.Len() > 0 {
			return out, nil
		}
	}
}

type bufPuller struct {
	vecs   []vector.Any
	EOS    bool
	puller vector.Puller
}

func (b *bufPuller) Pull(done bool) (vector.Any, error) {
	if done {
		if !b.EOS {
			return b.puller.Pull(done)
		}
		return nil, nil
	}
	if len(b.vecs) > 0 {
		vec := b.vecs[0]
		b.vecs = b.vecs[1:]
		return vec, nil
	}
	if b.EOS {
		return nil, nil
	}
	return b.puller.Pull(false)
}

func hashKey(val super.Value) string {
	return string(binary.LittleEndian.AppendUint32(val.Bytes(), uint32(val.Type().ID())))
}

func vectorValue(b *zcode.Builder, vec vector.Any, slot uint32) super.Value {
	vec.Serialize(b, slot)
	bytes := b.Bytes().Body()
	if dynVec, ok := vec.(*vector.Dynamic); ok {
		return super.NewValue(dynVec.TypeOf(slot), bytes)
	}
	return super.NewValue(vec.Type(), bytes)
}
