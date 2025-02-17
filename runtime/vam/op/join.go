package op

import (
	"encoding/binary"

	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/join"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type Join struct {
	zctx     *super.Context
	anti     bool
	inner    bool
	left     vector.Puller
	right    vector.Puller
	leftKey  expr.Evaluator
	rightKey expr.Evaluator

	cutter  *samexpr.Cutter
	splicer *join.RecordSplicer
	table   map[string][]super.Value
}

func NewJoin(zctx *super.Context, anti, inner bool, left, right vector.Puller, leftKey, rightKey expr.Evaluator, lhs []*samexpr.Lval, rhs []samexpr.Evaluator) *Join {
	return &Join{
		zctx:     zctx,
		anti:     anti,
		inner:    inner,
		left:     left,
		right:    right,
		leftKey:  leftKey,
		rightKey: rightKey,
		cutter:   samexpr.NewCutter(zctx, lhs, rhs),
		splicer:  join.NewRecordSplicer(zctx),
	}
}

func (j *Join) Pull(done bool) (vector.Any, error) {
	if done {
		_, err := j.left.Pull(true)
		if err == nil {
			_, err = j.right.Pull(true)
		}
		return nil, err
	}

	// Build
	if j.table == nil {
		j.table = map[string][]super.Value{}
		var keyBuilder, valBuilder zcode.Builder
		for {
			vec, err := j.right.Pull(false)
			if err != nil {
				return nil, err
			}
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
				j.table[key] = append(j.table[key], vectorValue(&valBuilder, vec, i))
			}
		}
	}

	// Probe
	for {
		leftVec, err := j.left.Pull(false)
		if err != nil {
			return nil, err
		}
		if leftVec == nil {
			return nil, nil
		}
		leftKeyVec := j.leftKey.Eval(leftVec)
		var keyBuilder, valBuilder zcode.Builder
		b := vector.NewDynamicBuilder()
		for i := range leftVec.Len() {
			keyBuilder.Truncate()
			keyVal := vectorValue(&keyBuilder, leftKeyVec, i)
			if keyVal.IsMissing() {
				continue
			}
			key := hashKey(keyVal)
			valBuilder.Truncate()
			leftVal := vectorValue(&valBuilder, leftVec, i)
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
