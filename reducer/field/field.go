package field

import (
	"github.com/mccanne/zq/expr"
	"github.com/mccanne/zq/reducer"
	"github.com/mccanne/zq/zng"
)

type Streamfn interface {
	Result() zng.Value
	Consume(zng.Value) error
}

type FieldProto struct {
	target   string
	op       string
	resolver expr.FieldExprResolver
}

func (fp *FieldProto) Target() string {
	return fp.target
}

func (fp *FieldProto) Instantiate(rec *zng.Record) reducer.Interface {
	v := fp.resolver(rec)
	if v.Type == nil {
		v.Type = zng.TypeNull
	}
	return &FieldReducer{op: fp.op, resolver: fp.resolver, typ: v.Type}
}

func NewFieldProto(target string, resolver expr.FieldExprResolver, op string) *FieldProto {
	return &FieldProto{target, op, resolver}
}

type FieldReducer struct {
	reducer.Reducer
	op       string
	resolver expr.FieldExprResolver
	typ      zng.Type
	fn       Streamfn
}

func (fr *FieldReducer) Result() zng.Value {
	if fr.fn == nil {
		return zng.Value{Type: fr.typ, Bytes: nil}
	}
	return fr.fn.Result()
}

func (fr *FieldReducer) Consume(r *zng.Record) {
	// XXX for now, we create a new zng.Value everytime we operate on
	// a field.  this could be made more efficient by having each typed
	// reducer just parse the byte slice in the record without making a value...
	// XXX then we have Values in the zbuf.Record, we would first check the
	// Value element in the column--- this would all go in a new method of zbuf.Record
	val := fr.resolver(r)
	if val.Type == nil {
		fr.FieldNotFound++
		return
	}
	if val.Bytes == nil {
		return
	}
	if fr.fn == nil {
		switch val.Type.(type) {
		case *zng.TypeOfInt:
			fr.fn = NewIntStreamfn(fr.op)
		case *zng.TypeOfCount:
			fr.fn = NewCountStreamfn(fr.op)
		case *zng.TypeOfDouble:
			fr.fn = NewDoubleStreamfn(fr.op)
		case *zng.TypeOfInterval:
			fr.fn = NewIntervalStreamfn(fr.op)
		case *zng.TypeOfTime:
			fr.fn = NewTimeStreamfn(fr.op)
		default:
			fr.TypeMismatch++
			return
		}
	}
	if fr.fn.Consume(val) == zng.ErrTypeMismatch {
		fr.TypeMismatch++
	}
}
