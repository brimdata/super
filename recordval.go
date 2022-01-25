package zed

import (
	"errors"
	"math"

	"github.com/brimdata/zed/field"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zcode"
	"inet.af/netaddr"
)

var (
	ErrMissingField  = errors.New("record missing a field")
	ErrExtraField    = errors.New("record with extra field")
	ErrNotContainer  = errors.New("expected container type, got primitive")
	ErrNotPrimitive  = errors.New("expected primitive type, got container")
	ErrTypeIDInvalid = errors.New("zng type ID out of range")
	ErrBadValue      = errors.New("malformed zng value")
	ErrBadFormat     = errors.New("malformed zng record")
	ErrTypeMismatch  = errors.New("type/value mismatch")
)

// FieldIter returns a fieldIter iterator over the receiver's values.
func (r *Value) FieldIter() fieldIter {
	return fieldIter{
		stack: []iterInfo{{
			iter: r.Bytes.Iter(),
			typ:  TypeRecordOf(r.Type),
		}},
	}
}

func (r *Value) HasField(field string) bool {
	return TypeRecordOf(r.Type).HasField(field)
}

// Walk traverses a value in depth-first order, calling a
// Visitor on the way.
func (r *Value) Walk(rv Visitor) error {
	return Walk(r.Type, r.Bytes, rv)
}

// Slice returns the encoded zcode.Bytes corresponding to the indicated
// column or an error if a problem was encountered.
func (r *Value) Slice(column int) (zcode.Bytes, error) {
	var zv zcode.Bytes
	for i, it := 0, r.Bytes.Iter(); i <= column; i++ {
		if it.Done() {
			return nil, ErrMissing
		}
		zv = it.Next()
	}
	return zv, nil
}

func (r *Value) Columns() []Column {
	return TypeRecordOf(r.Type).Columns
}

// Value returns the indicated column as a Value.  If the column doesn't
// exist or another error occurs, the nil Value is returned.
func (r *Value) ValueByColumn(col int) Value {
	zv, err := r.Slice(col)
	if err != nil {
		return Value{}
	}
	return Value{r.Columns()[col].Type, zv}
}

func (r *Value) ValueByField(field string) (Value, error) {
	col, ok := r.ColumnOfField(field)
	if !ok {
		return Value{}, ErrMissing
	}
	return r.ValueByColumn(col), nil
}

func (r *Value) ColumnOfField(field string) (int, bool) {
	return TypeRecordOf(r.Type).ColumnOfField(field)
}

func (r *Value) TypeOfColumn(col int) Type {
	return TypeRecordOf(r.Type).Columns[col].Type
}

func (r *Value) Access(field string) (Value, error) {
	col, ok := r.ColumnOfField(field)
	if !ok {
		return Value{}, ErrMissing
	}
	return r.ValueByColumn(col), nil
}

func (r *Value) Deref(path field.Path) (Value, error) {
	v := *r
	for _, f := range path {
		typ := TypeRecordOf(v.Type)
		if typ == nil {
			return Value{}, errors.New("field access on non-record value")
		}
		var err error
		v, err = NewValue(typ, v.Bytes).Access(f)
		if err != nil {
			return Value{}, err
		}
	}
	return v, nil
}

func (r *Value) AccessString(field string) (string, error) {
	v, err := r.Access(field)
	if err != nil {
		return "", err
	}
	if TypeUnder(v.Type) == TypeString {
		return DecodeString(v.Bytes), nil
	}
	return "", ErrTypeMismatch
}

func (r *Value) AccessBool(field string) (bool, error) {
	v, err := r.Access(field)
	if err != nil {
		return false, err
	}
	if _, ok := TypeUnder(v.Type).(*TypeOfBool); !ok {
		return false, ErrTypeMismatch
	}
	return DecodeBool(v.Bytes), nil
}

func (r *Value) AccessInt(field string) (int64, error) {
	v, err := r.Access(field)
	if err != nil {
		return 0, err
	}
	switch TypeUnder(v.Type).(type) {
	case *TypeOfUint8, *TypeOfUint16, *TypeOfUint32:
		return int64(DecodeUint(v.Bytes)), nil
	case *TypeOfUint64:
		v := DecodeUint(v.Bytes)
		if v > math.MaxInt64 {
			return 0, errors.New("conversion from uint64 to signed int results in overflow")
		}
		return int64(v), err
	case *TypeOfInt8, *TypeOfInt16, *TypeOfInt32, *TypeOfInt64:
		return DecodeInt(v.Bytes), nil
	}
	return 0, ErrTypeMismatch
}

func (r *Value) AccessIP(field string) (netaddr.IP, error) {
	v, err := r.Access(field)
	if err != nil {
		return netaddr.IP{}, err
	}
	if _, ok := TypeUnder(v.Type).(*TypeOfIP); !ok {
		return netaddr.IP{}, ErrTypeMismatch
	}
	return DecodeIP(v.Bytes), nil
}

func (r *Value) AccessTime(field string) (nano.Ts, error) {
	v, err := r.Access(field)
	if err != nil {
		return 0, err
	}
	if _, ok := TypeUnder(v.Type).(*TypeOfTime); !ok {
		return 0, ErrTypeMismatch
	}
	return DecodeTime(v.Bytes), nil
}

func (r *Value) AccessTimeByColumn(colno int) (nano.Ts, error) {
	zv, err := r.Slice(colno)
	if err != nil {
		return 0, err
	}
	return DecodeTime(zv), nil
}
