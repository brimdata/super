package expr

import (
	"encoding/binary"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/agg"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type Aggregator struct {
	pattern  agg.Pattern
	Name     string
	distinct bool
	Expr     Evaluator
	Where    Evaluator
}

func NewAggregator(name string, distinct bool, expr Evaluator, where Evaluator) (*Aggregator, error) {
	pattern, err := agg.NewPattern(name, expr != nil)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		// Count is the only that has no argument so we just return
		// true so it counts each value encountered.
		expr = NewLiteral(super.True)
	}
	return &Aggregator{
		pattern:  pattern,
		Name:     name,
		distinct: distinct,
		Expr:     expr,
		Where:    where,
	}, nil
}

func (a *Aggregator) Eval(this vector.Any) vector.Any {
	vec := a.Expr.Eval(this)
	if a.Where == nil {
		return vec
	}
	return vector.Apply(true, a.apply, vec, a.Where.Eval(this))
}

func (a *Aggregator) apply(args ...vector.Any) vector.Any {
	vec, where := args[0], args[1]
	bools, _ := BoolMask(where)
	if bools.IsEmpty() {
		// everything is filtered.
		return vector.NewConst(super.NewValue(vec.Type(), nil), vec.Len(), nil)
	}
	bools.Flip(0, uint64(vec.Len()))
	if !bools.IsEmpty() {
		nulls := vector.NewBoolEmpty(vec.Len(), nil)
		bools.WriteDenseTo(nulls.Bits)
		if origNulls := vector.NullsOf(vec); origNulls != nil {
			nulls = vector.Or(nulls, origNulls)
		}
		vec = vector.CopyAndSetNulls(vec, nulls)
	}
	return vec
}

func (a *Aggregator) NewFunction() agg.Func {
	f := a.pattern()
	if a.distinct {
		f = &distinct{f, map[string]struct{}{}}
	}
	return f
}

type distinct struct {
	agg.Func
	seen map[string]struct{}
}

func (d *distinct) Consume(vec vector.Any) {
	id := vec.Type().ID()
	var index []uint32
	var b zcode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		buf := binary.AppendVarint(b.Bytes(), int64(id))
		if _, ok := d.seen[string(buf)]; ok {
			continue
		}
		d.seen[string(buf)] = struct{}{}
		index = append(index, i)
	}
	if len(index) < int(vec.Len()) {
		vec = vector.NewView(vec, index)
	}
	d.Func.Consume(vec)

}
