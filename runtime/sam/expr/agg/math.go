package agg

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/anymath"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/sup"
)

type consumer interface {
	result() super.Value
	consume(super.Value)
	typ() super.Type
}

type mathReducer struct {
	function *anymath.Function
	hasval   bool
	math     consumer
}

var _ Function = (*mathReducer)(nil)

func newMathReducer(f *anymath.Function) *mathReducer {
	return &mathReducer{function: f}
}

func (m *mathReducer) Result(zctx *super.Context) super.Value {
	if !m.hasval {
		if m.math == nil {
			return super.Null
		}
		return super.NewValue(m.math.typ(), nil)
	}
	return m.math.result()
}

func (m *mathReducer) Consume(val super.Value) {
	m.consumeVal(val)
}

func (m *mathReducer) consumeVal(val super.Value) {
	var id int
	if m.math != nil {
		var err error
		id, err = coerce.Promote(super.NewValue(m.math.typ(), nil), val)
		if err != nil {
			// Skip invalid values.
			return
		}
	} else {
		id = val.Type().ID()
	}
	if m.math == nil || m.math.typ().ID() != id {
		state := super.Null
		if m.math != nil {
			state = m.math.result()
		}
		switch id {
		case super.IDInt8, super.IDInt16, super.IDInt32, super.IDInt64:
			m.math = NewInt64(m.function, state)
		case super.IDUint8, super.IDUint16, super.IDUint32, super.IDUint64:
			m.math = NewUint64(m.function, state)
		case super.IDFloat16, super.IDFloat32, super.IDFloat64:
			m.math = NewFloat64(m.function, state)
		case super.IDDuration:
			m.math = NewDuration(m.function, state)
		case super.IDTime:
			m.math = NewTime(m.function, state)
		default:
			// Ignore types we can't handle.
			return
		}
	}
	if val.IsNull() {
		return
	}
	m.hasval = true
	m.math.consume(val)
}

func (m *mathReducer) ResultAsPartial(*super.Context) super.Value {
	return m.Result(nil)
}

func (m *mathReducer) ConsumeAsPartial(val super.Value) {
	m.consumeVal(val)
}

type Float64 struct {
	state    float64
	function anymath.Float64
}

func NewFloat64(f *anymath.Function, val super.Value) *Float64 {
	state := f.Init.Float64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToFloat(val, super.TypeFloat64)
		if !ok {
			panicCoercionFail(super.TypeFloat64, val.Type())
		}
	}
	return &Float64{
		state:    state,
		function: f.Float64,
	}
}

func (f *Float64) result() super.Value {
	return super.NewFloat64(f.state)
}

func (f *Float64) consume(val super.Value) {
	if v, ok := coerce.ToFloat(val, super.TypeFloat64); ok {
		f.state = f.function(f.state, v)
	}
}

func (f *Float64) typ() super.Type { return super.TypeFloat64 }

type Int64 struct {
	state    int64
	function anymath.Int64
}

func NewInt64(f *anymath.Function, val super.Value) *Int64 {
	state := f.Init.Int64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToInt(val, super.TypeInt64)
		if !ok {
			panicCoercionFail(super.TypeInt64, val.Type())
		}
	}
	return &Int64{
		state:    state,
		function: f.Int64,
	}
}

func (i *Int64) result() super.Value {
	return super.NewInt64(i.state)
}

func (i *Int64) consume(val super.Value) {
	if v, ok := coerce.ToInt(val, super.TypeInt64); ok {
		i.state = i.function(i.state, v)
	}
}

func (f *Int64) typ() super.Type { return super.TypeInt64 }

type Uint64 struct {
	state    uint64
	function anymath.Uint64
}

func NewUint64(f *anymath.Function, val super.Value) *Uint64 {
	state := f.Init.Uint64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToUint(val, super.TypeUint64)
		if !ok {
			panicCoercionFail(super.TypeUint64, val.Type())
		}
	}
	return &Uint64{
		state:    state,
		function: f.Uint64,
	}
}

func (u *Uint64) result() super.Value {
	return super.NewUint64(u.state)
}

func (u *Uint64) consume(val super.Value) {
	if v, ok := coerce.ToUint(val, super.TypeUint64); ok {
		u.state = u.function(u.state, v)
	}
}

func (f *Uint64) typ() super.Type { return super.TypeUint64 }

type Duration struct {
	state    int64
	function anymath.Int64
}

func NewDuration(f *anymath.Function, val super.Value) *Duration {
	state := f.Init.Int64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToInt(val, super.TypeDuration)
		if !ok {
			panicCoercionFail(super.TypeDuration, val.Type())
		}
	}
	return &Duration{
		state:    state,
		function: f.Int64,
	}
}

func (d *Duration) result() super.Value {
	return super.NewDuration(nano.Duration(d.state))
}

func (d *Duration) consume(val super.Value) {
	if v, ok := coerce.ToInt(val, super.TypeDuration); ok {
		d.state = d.function(d.state, v)
	}
}

func (f *Duration) typ() super.Type { return super.TypeDuration }

type Time struct {
	state    nano.Ts
	function anymath.Int64
}

func NewTime(f *anymath.Function, val super.Value) *Time {
	state := f.Init.Int64
	if !val.IsNull() {
		var ok bool
		state, ok = coerce.ToInt(val, super.TypeTime)
		if !ok {
			panicCoercionFail(super.TypeTime, val.Type())
		}
	}
	return &Time{
		state:    nano.Ts(state),
		function: f.Int64,
	}
}

func (t *Time) result() super.Value {
	return super.NewTime(t.state)
}

func (t *Time) consume(val super.Value) {
	if v, ok := coerce.ToInt(val, super.TypeTime); ok {
		t.state = nano.Ts(t.function(int64(t.state), v))
	}
}

func (f *Time) typ() super.Type { return super.TypeTime }

func panicCoercionFail(to, from super.Type) {
	panic(fmt.Sprintf("internal aggregation error: cannot coerce %s to %s", sup.String(from), sup.String(to)))
}
