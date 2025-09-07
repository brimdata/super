package expr

import (
	"errors"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/scode"
)

type This struct{}

func (*This) Eval(this super.Value) super.Value {
	return this
}

type DotExpr struct {
	sctx         *super.Context
	record       Evaluator
	field        string
	fieldIndices []int
}

func NewDotExpr(sctx *super.Context, record Evaluator, field string) *DotExpr {
	return &DotExpr{
		sctx:   sctx,
		record: record,
		field:  field,
	}
}

func NewDottedExpr(sctx *super.Context, f field.Path) Evaluator {
	ret := Evaluator(&This{})
	for _, name := range f {
		ret = NewDotExpr(sctx, ret, name)
	}
	return ret
}

func (d *DotExpr) Eval(this super.Value) super.Value {
	val := d.record.Eval(this).Under()
	// Cases are ordered by decreasing expected frequency.
	switch typ := val.Type().(type) {
	case *super.TypeRecord:
		i, ok := d.fieldIndex(typ)
		if !ok {
			return d.sctx.Missing()
		}
		bytes, idx := getNthFromContainer(val.Bytes(), i)
		if idx < 0 {
			return d.sctx.Missing()
		}
		return super.NewValue(typ.Fields[i].Type, bytes)
	case *super.TypeMap:
		return indexMap(d.sctx, typ, val.Bytes(), super.NewString(d.field))
	case *super.TypeOfType:
		return d.evalTypeOfType(val.Bytes())
	}
	return d.sctx.Missing()
}

func (d *DotExpr) fieldIndex(typ *super.TypeRecord) (int, bool) {
	id := typ.ID()
	if id >= len(d.fieldIndices) {
		d.fieldIndices = slices.Grow(d.fieldIndices[:0], id+1)[:id+1]
	}
	if i := d.fieldIndices[id]; i > 0 {
		return i - 1, true
	} else if i < 0 {
		return 0, false
	}
	i, ok := typ.IndexOfField(d.field)
	if ok {
		d.fieldIndices[id] = i + 1
	} else {
		d.fieldIndices[id] = -1
	}
	return i, ok
}

func (d *DotExpr) evalTypeOfType(b scode.Bytes) super.Value {
	if b == nil {
		return super.NewValue(super.TypeType, nil)
	}
	typ, _ := d.sctx.DecodeTypeValue(b)
	if typ, ok := super.TypeUnder(typ).(*super.TypeRecord); ok {
		if typ, ok := typ.TypeOfField(d.field); ok {
			return d.sctx.LookupTypeValue(typ)
		}
	}
	return d.sctx.Missing()
}

// DotExprToString returns text for the Evaluator assuming it's a field expr.
func DotExprToString(e Evaluator) (string, error) {
	f, err := DotExprToField(e)
	if err != nil {
		return "", err
	}
	return f.String(), nil
}

func DotExprToField(e Evaluator) (field.Path, error) {
	switch e := e.(type) {
	case *This:
		return field.Path{}, nil
	case *DotExpr:
		lhs, err := DotExprToField(e.record)
		if err != nil {
			return nil, err
		}
		return append(lhs, e.field), nil
	case *Literal:
		return field.Path{e.val.String()}, nil
	case *Index:
		lhs, err := DotExprToField(e.container)
		if err != nil {
			return nil, err
		}
		rhs, err := DotExprToField(e.index)
		if err != nil {
			return nil, err
		}
		return append(lhs, rhs...), nil
	}
	return nil, errors.New("not a field")
}
