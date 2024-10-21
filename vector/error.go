package vector

import (
	"encoding/binary"

	zed "github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Error struct {
	Typ   *zed.TypeError
	Vals  Any
	Nulls *Bool
}

var _ Any = (*Error)(nil)

// XXX we shouldn't create empty fields... this was the old design, now
// we create the entire vector structure and page in leaves, offsets, etc on demand
func NewError(typ *zed.TypeError, vals Any, nulls *Bool) *Error {
	return &Error{Typ: typ, Vals: vals, Nulls: nulls}
}

func (e *Error) Type() zed.Type {
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

func (e *Error) AppendKey(bytes []byte, slot uint32) []byte {
	bytes = binary.NativeEndian.AppendUint32(bytes, uint32(e.Typ.ID()))
	if e.Nulls.Value(slot) {
		return append(bytes, 0)
	}
	return e.Vals.AppendKey(bytes, slot)
}

func NewStringError(zctx *zed.Context, msg string, len uint32) *Error {
	vals := NewConst(zed.NewString(msg), len, nil)
	return &Error{Typ: zctx.LookupTypeError(zed.TypeString), Vals: vals}
}

func NewMissing(zctx *zed.Context, len uint32) *Error {
	return NewStringError(zctx, "missing", len)
}

func NewWrappedError(zctx *zed.Context, msg string, val Any) *Error {
	msgVec := NewConst(zed.NewString(msg), val.Len(), nil)
	return NewVecWrappedError(zctx, msgVec, val)
}

func NewVecWrappedError(zctx *zed.Context, msg Any, val Any) *Error {
	recType := zctx.MustLookupTypeRecord([]zed.Field{
		{Name: "message", Type: msg.Type()},
		{Name: "on", Type: val.Type()},
	})
	rval := NewRecord(recType, []Any{msg, val}, val.Len(), nil)
	return &Error{Typ: zctx.LookupTypeError(recType), Vals: rval}
}
