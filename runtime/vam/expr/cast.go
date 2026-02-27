package expr

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

func NewLiteralCast(sctx *super.Context, expr Evaluator, literal *Literal) (Evaluator, error) {
	typeVal := literal.val
	switch typeVal.Type().ID() {
	case super.IDType:
		typ, err := sctx.LookupByValue(typeVal.Bytes())
		if err != nil {
			return nil, err
		}
		if typ.ID() >= super.IDTypeComplex {
			return nil, fmt.Errorf("cast: casting to type %s not currently supported in vector runtime", sup.FormatType(typ))
		}
		return &casterPrimitive{sctx, expr, typ}, nil
	case super.IDString:
		name := super.DecodeString(typeVal.Bytes())
		if _, err := super.NewContext().LookupTypeNamed(name, super.TypeNull); err != nil {
			return nil, err
		}
		return &casterNamedType{sctx, expr, name}, nil
	default:
		return nil, fmt.Errorf("cast type argument is not a type: %s", sup.FormatValue(typeVal))
	}
}

type casterPrimitive struct {
	sctx *super.Context
	expr Evaluator
	typ  super.Type
}

func (c *casterPrimitive) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		return cast.To(c.sctx, vecs[0], c.typ)
	}, c.expr.Eval(this))
}

type casterNamedType struct {
	sctx *super.Context
	expr Evaluator
	name string
}

func (c *casterNamedType) Eval(this vector.Any) vector.Any {
	return vector.Apply(false, c.eval, c.expr.Eval(this))
}

func (c *casterNamedType) eval(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	if vec.Kind() == vector.KindError {
		return vec
	}
	vec = vector.Under(vec)
	named, err := c.sctx.LookupTypeNamed(c.name, vec.Type())
	if err != nil {
		return vector.NewStringError(c.sctx, err.Error(), vec.Len())
	}
	return vector.NewNamed(named, vec)
}
