package expr

import (
	"errors"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/zson"
)

type Lval struct {
	Elems []LvalElem
	cache field.Path
}

func NewLval(evals []LvalElem) *Lval {
	return &Lval{Elems: evals}
}

// Eval returns the path of the lval.
func (l *Lval) Eval(ectx Context, this super.Value) (field.Path, error) {
	l.cache = l.cache[:0]
	for _, e := range l.Elems {
		name, err := e.Eval(ectx, this)
		if err != nil {
			return nil, err
		}
		l.cache = append(l.cache, name)
	}
	return l.cache, nil
}

// Path returns the receiver's path.  Path returns false when the receiver
// contains a dynamic element.
func (l *Lval) Path() (field.Path, bool) {
	var path field.Path
	for _, e := range l.Elems {
		s, ok := e.(*StaticLvalElem)
		if !ok {
			return nil, false
		}
		path = append(path, s.Name)
	}
	return path, true
}

type LvalElem interface {
	Eval(ectx Context, this super.Value) (string, error)
}

type StaticLvalElem struct {
	Name string
}

func (l *StaticLvalElem) Eval(_ Context, _ super.Value) (string, error) {
	return l.Name, nil
}

type ExprLvalElem struct {
	caster Evaluator
	eval   Evaluator
}

func NewExprLvalElem(zctx *super.Context, e Evaluator) *ExprLvalElem {
	return &ExprLvalElem{
		eval:   e,
		caster: LookupPrimitiveCaster(zctx, super.TypeString),
	}
}

func (l *ExprLvalElem) Eval(ectx Context, this super.Value) (string, error) {
	val := l.eval.Eval(ectx, this)
	if val.IsError() {
		return "", lvalErr(ectx, val)
	}
	if !val.IsString() {
		if val = l.caster.Eval(ectx, val); val.IsError() {
			return "", errors.New("field reference is not a string")
		}
	}
	return val.AsString(), nil
}

func lvalErr(ectx Context, errVal super.Value) error {
	val := super.NewValue(errVal.Type().(*super.TypeError).Type, errVal.Bytes())
	if val.IsString() {
		return errors.New(val.AsString())
	}
	return errors.New(zson.FormatValue(val))
}
