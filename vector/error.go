package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Error struct {
	l      *lock
	loader NullsLoader
	Typ    *super.TypeError
	Vals   Any
	nulls  bitvec.Bits
}

var _ Any = (*Error)(nil)

// XXX we shouldn't create empty fields... this was the old design, now
// we create the entire vector structure and page in leaves, offsets, etc on demand
func NewError(typ *super.TypeError, vals Any, nulls bitvec.Bits) *Error {
	return &Error{Typ: typ, Vals: vals, nulls: nulls}
}

func NewLazyError(typ *super.TypeError, loader NullsLoader, vals Any) *Error {
	e := &Error{Typ: typ, Vals: vals, loader: loader}
	e.l = newLock(e)
	return e
}

func (e *Error) Type() super.Type {
	return e.Typ
}

func (e *Error) Len() uint32 {
	return e.Vals.Len()
}

func (e *Error) load() {
	e.nulls = e.loader.Load()
}

func (e *Error) Nulls() bitvec.Bits {
	e.l.check()
	return e.nulls
}

func (e *Error) SetNulls(nulls bitvec.Bits) {
	e.nulls = nulls
}

func (e *Error) Serialize(b *zcode.Builder, slot uint32) {
	if e.Nulls().IsSet(slot) {
		b.Append(nil)
		return
	}
	e.Vals.Serialize(b, slot)

}

func NewStringError(sctx *super.Context, msg string, len uint32) *Error {
	vals := NewConst(super.NewString(msg), len, bitvec.Zero)
	return &Error{Typ: sctx.LookupTypeError(super.TypeString), Vals: vals}
}

func NewMissing(sctx *super.Context, len uint32) *Error {
	return NewStringError(sctx, "missing", len)
}

func NewWrappedError(sctx *super.Context, msg string, val Any) *Error {
	msgVec := NewConst(super.NewString(msg), val.Len(), bitvec.Zero)
	return NewVecWrappedError(sctx, msgVec, val)
}

func NewVecWrappedError(sctx *super.Context, msg Any, val Any) *Error {
	recType := sctx.MustLookupTypeRecord([]super.Field{
		{Name: "message", Type: msg.Type()},
		{Name: "on", Type: val.Type()},
	})
	rval := NewRecord(recType, []Any{msg, val}, val.Len(), bitvec.Zero)
	return &Error{Typ: sctx.LookupTypeError(recType), Vals: rval}
}
