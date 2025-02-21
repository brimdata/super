package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Error struct {
	Typ   *super.TypeError
	Vals  Any
	Nulls *Bool
}

var _ Any = (*Error)(nil)

// XXX we shouldn't create empty fields... this was the old design, now
// we create the entire vector structure and page in leaves, offsets, etc on demand
func NewError(typ *super.TypeError, vals Any, nulls *Bool) *Error {
	return &Error{Typ: typ, Vals: vals, Nulls: nulls}
}

func (e *Error) Type() super.Type {
	return e.Typ
}

func (e *Error) Len() uint32 {
	return e.Vals.Len()
}

func (e *Error) Serialize(b *zcode.Builder, slot uint32) {
	if e.Nulls.Value(slot) {
		b.Append(nil)
		return
	}
	e.Vals.Serialize(b, slot)

}

func NewStringError(zctx *super.Context, msg string, len uint32) *Error {
	vals := NewConst(super.NewString(msg), len, nil)
	return &Error{Typ: zctx.LookupTypeError(super.TypeString), Vals: vals}
}

func NewMissing(zctx *super.Context, len uint32) *Error {
	return NewStringError(zctx, "missing", len)
}

func NewWrappedError(zctx *super.Context, msg string, val Any) *Error {
	msgVec := NewConst(super.NewString(msg), val.Len(), nil)
	return NewVecWrappedError(zctx, msgVec, val)
}

func NewVecWrappedError(zctx *super.Context, msg Any, val Any) *Error {
	recType := zctx.MustLookupTypeRecord([]super.Field{
		{Name: "message", Type: msg.Type()},
		{Name: "on", Type: val.Type()},
	})
	rval := NewRecord(recType, []Any{msg, val}, val.Len(), nil)
	return &Error{Typ: zctx.LookupTypeError(recType), Vals: rval}
}
