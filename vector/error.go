package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Error struct {
	Typ   *super.TypeError
	Vals  Any
	Nulls bitvec.Bits
}

var _ Any = (*Error)(nil)

// XXX we shouldn't create empty fields... this was the old design, now
// we create the entire vector structure and page in leaves, offsets, etc on demand
func NewError(typ *super.TypeError, vals Any, nulls bitvec.Bits) *Error {
	return &Error{Typ: typ, Vals: vals, Nulls: nulls}
}

func (*Error) Kind() Kind {
	return KindError
}

func (e *Error) Type() super.Type {
	return e.Typ
}

func (e *Error) Len() uint32 {
	return e.Vals.Len()
}

func (e *Error) Serialize(b *scode.Builder, slot uint32) {
	if e.Nulls.IsSet(slot) {
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
