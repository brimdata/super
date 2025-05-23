package agg

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type consumer interface {
	result() super.Value
	consume(vector.Any)
	typ() super.Type
}

type mathReducer struct {
	function  *mathFunc
	hasval    bool
	math      consumer
	stringErr bool
}

func newMathReducer(f *mathFunc) *mathReducer {
	return &mathReducer{function: f}
}

var _ Func = (*mathReducer)(nil)

func (m *mathReducer) Result(sctx *super.Context) super.Value {
	if m.stringErr {
		return sctx.NewErrorf("mixture of string and numeric values")
	}
	if !m.hasval {
		if m.math == nil {
			return super.Null
		}
		return super.NewValue(m.math.typ(), nil)
	}
	return m.math.result()
}

func (m *mathReducer) Consume(vec vector.Any) {
	if m.stringErr {
		return
	}
	vec = vector.Under(vec)
	typ := vec.Type()
	if typ == super.TypeString || (m.math != nil && m.math.typ() == super.TypeString) {
		m.consumeString(vec)
		return
	}
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
	if vec = trimNulls(vec); vec.Len() == 0 {
		return
	}
	m.hasval = true
	m.math.consume(vec)
}

func (m *mathReducer) consumeString(vec vector.Any) {
	if m.math == nil {
		m.math = newReduceString(m.function)
	}
	aid := vec.Type().ID()
	bid := m.math.typ().ID()
	if aid == super.IDString && bid == super.IDString {
		if vec = trimNulls(vec); vec.Len() == 0 {
			return
		}
		m.hasval = true
		m.math.consume(vec)
	} else if super.IsNumber(aid) || super.IsNumber(bid) {
		m.stringErr = true
	}
}

func (m *mathReducer) ConsumeAsPartial(vec vector.Any) {
	m.Consume(vec)
}

func (m *mathReducer) ResultAsPartial(*super.Context) super.Value {
	return m.Result(nil)
}

func trimNulls(vec vector.Any) vector.Any {
	if c, ok := vec.(*vector.Const); ok && c.Value().IsNull() {
		return vector.NewConst(super.Null, 0, bitvec.Zero)
	}
	nulls := vector.NullsOf(vec)
	if nulls.IsZero() {
		return vec
	}
	var index []uint32
	for i := range nulls.Len() {
		if nulls.IsSet(i) {
			index = append(index, i)
		}
	}
	switch uint32(len(index)) {
	case vec.Len():
		return vector.NewConst(super.Null, 0, bitvec.Zero)
	case 0:
		return vec
	default:
		return vector.ReversePick(vec, index)
	}
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

type reduceString struct {
	state    string
	hasval   bool
	function funcString
}

func newReduceString(f *mathFunc) *reduceString {
	return &reduceString{function: f.funcString}
}

func (s *reduceString) result() super.Value {
	if s.function == nil {
		return super.Null
	}
	return super.NewString(s.state)
}

func (s *reduceString) consume(vec vector.Any) {
	if s.function == nil {
		return
	}
	if !s.hasval {
		s.state, _ = vector.StringValue(vec, 0)
		s.hasval = true
	}
	s.state = s.function(s.state, vec)
}

func (s *reduceString) typ() super.Type { return super.TypeString }

func panicCoercionFail(to, from super.Type) {
	panic(fmt.Sprintf("internal aggregation error: cannot coerce %s to %s", sup.String(from), sup.String(to)))
}
