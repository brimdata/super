package agg

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type consumer interface {
	result() super.Value
	consume(vector.Any)
	typ() super.Type
}

type mathReducer struct {
	function *mathFunc
	hasval   bool
	math     consumer
}

func newMathReducer(f *mathFunc) *mathReducer {
	return &mathReducer{function: f}
}

var _ Func = (*mathReducer)(nil)

func (m *mathReducer) Result(*super.Context) super.Value {
	if !m.hasval {
		if m.math == nil {
			return super.Null
		}
		return super.NewValue(m.math.typ(), nil)
	}
	return m.math.result()
}

func (m *mathReducer) Consume(vec vector.Any) {
	vec = vector.Under(vec)
	typ := vec.Type()
	var id int
	if m.math != nil {
		var err error
		id, err = coerce.Promote(super.NewValue(m.math.typ(), nil), super.NewValue(typ, nil))
		if err != nil {
			// Skip invalid values.
			return
		}
	} else {
		id = typ.ID()
	}
	if m.math == nil || m.math.typ().ID() != id {
		state := super.Null
		if m.math != nil {
			state = m.math.result()
		}
		switch id {
		case super.IDUint8, super.IDUint16, super.IDUint32, super.IDUint64:
			m.math = newReduceUint64(m.function, state)
		case super.IDInt8, super.IDInt16, super.IDInt32, super.IDInt64:
			m.math = newReduceInt64(m.function, state, super.TypeInt64)
		case super.IDDuration:
			m.math = newReduceInt64(m.function, state, super.TypeDuration)
		case super.IDTime:
			m.math = newReduceInt64(m.function, state, super.TypeTime)
		case super.IDFloat16, super.IDFloat32, super.IDFloat64:
			m.math = newReduceFloat64(m.function, state)
		default:
			// Ignore types we can't handle.
			return
		}
	}
	if isNull(vec) {
		return
	}
	m.hasval = true
	m.math.consume(vec)
}

func (m *mathReducer) ConsumeAsPartial(vec vector.Any) {
	m.Consume(vec)
}

func (m *mathReducer) ResultAsPartial(*super.Context) super.Value {
	return m.Result(nil)
}

func isNull(vec vector.Any) bool {
	if c, ok := vec.(*vector.Const); ok && c.Value().IsNull() {
		return true
	}
	if nulls := vector.NullsOf(vec); nulls != nil {
		// XXX There's probably a faster way of doing this check. Like check if
		// each uint64 is MaxUint64 but you run across the problem when the len
		// truncates a uint64.
		for i := range vec.Len() {
			if !nulls.Value(i) {
				return false
			}
		}
		return true
	}
	return false
}

type reduceFloat64 struct {
	state    float64
	function funcFloat64
}

func newReduceFloat64(f *mathFunc, val super.Value) *reduceFloat64 {
	state := f.Init.Float64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToFloat(val, super.TypeFloat64)
		if !ok {
			panicCoercionFail(super.TypeFloat64, val.Type())
		}
	}
	return &reduceFloat64{
		state:    state,
		function: f.funcFloat64,
	}
}

func (f *reduceFloat64) consume(vec vector.Any) {
	f.state = f.function(f.state, vec)
}

func (f *reduceFloat64) result() super.Value {
	return super.NewFloat64(f.state)
}

func (f *reduceFloat64) typ() super.Type { return super.TypeFloat64 }

type reduceInt64 struct {
	state    int64
	outtyp   super.Type
	function funcInt64
}

func newReduceInt64(f *mathFunc, val super.Value, typ super.Type) *reduceInt64 {
	state := f.Init.Int64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToInt(val, typ)
		if !ok {
			panicCoercionFail(super.TypeInt64, val.Type())
		}
	}
	return &reduceInt64{
		state:    state,
		outtyp:   typ,
		function: f.funcInt64,
	}
}

func (i *reduceInt64) result() super.Value {
	return super.NewInt(i.outtyp, i.state)
}

func (i *reduceInt64) consume(vec vector.Any) {
	i.state = i.function(i.state, vec)
}

func (f *reduceInt64) typ() super.Type { return super.TypeInt64 }

type reduceUint64 struct {
	state    uint64
	function funcUint64
}

func newReduceUint64(f *mathFunc, val super.Value) *reduceUint64 {
	state := f.Init.Uint64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToUint(val, super.TypeUint64)
		if !ok {
			panicCoercionFail(super.TypeUint64, val.Type())
		}
	}
	return &reduceUint64{
		state:    state,
		function: f.funcUint64,
	}
}

func (u *reduceUint64) result() super.Value {
	return super.NewUint64(u.state)
}

func (u *reduceUint64) consume(vec vector.Any) {
	u.state = u.function(u.state, vec)
}

func (f *reduceUint64) typ() super.Type { return super.TypeUint64 }

func panicCoercionFail(to, from super.Type) {
	panic(fmt.Sprintf("internal aggregation error: cannot coerce %s to %s", sup.String(from), sup.String(to)))
}
