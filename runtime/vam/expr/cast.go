package expr

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type literalCast struct {
	caster Evaluator
	expr   Evaluator
}

func NewLiteralCast(zctx *super.Context, expr Evaluator, literal *Literal) (Evaluator, error) {
	var c Evaluator
	typeVal := literal.val
	switch typeVal.Type().ID() {
	case super.IDType:
		typ, err := zctx.LookupByValue(typeVal.Bytes())
		if err != nil {
			return nil, err
		}
		if typ.ID() >= super.IDTypeComplex {
			return nil, fmt.Errorf("cast: casting to type %s not currently supported in vector runtime", sup.FormatType(typ))
		}
		c = &casterPrimitive{zctx, typ}
	case super.IDString:
		name := super.DecodeString(typeVal.Bytes())
		if _, err := super.NewContext().LookupTypeNamed(name, super.TypeNull); err != nil {
			return nil, err
		}
		c = &casterNamedType{zctx, name}
	default:
		return nil, fmt.Errorf("cast type argument is not a type: %s", sup.FormatValue(typeVal))
	}
	return &literalCast{c, expr}, nil
}

func (p *literalCast) Eval(vec vector.Any) vector.Any {
	return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		return p.caster.Eval(vecs[0])
	}, p.expr.Eval(vec))
}

type casterPrimitive struct {
	zctx *super.Context
	typ  super.Type
}

func (c *casterPrimitive) Eval(this vector.Any) vector.Any {
	return cast.To(c.zctx, this, c.typ)
}

type casterNamedType struct {
	zctx *super.Context
	name string
}

func (c *casterNamedType) Eval(this vector.Any) vector.Any {
	this = vector.Under(this)
	typ := this.Type()
	if typ.Kind() == super.ErrorKind {
		return this
	}
	named, err := c.zctx.LookupTypeNamed(c.name, typ)
	if err != nil {
		return vector.NewStringError(c.zctx, err.Error(), this.Len())
	}
	return vector.NewNamed(named, this)
}
