package kernel

import (
	"errors"
	"fmt"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/aggregate"
	"github.com/brimdata/super/zbuf"
)

func (b *Builder) compileAggregate(parent zbuf.Puller, a *dag.Aggregate) (*aggregate.Op, error) {
	b.resetResetters()
	keys, err := b.compileAssignments(a.Keys)
	if err != nil {
		return nil, err
	}
	names, reducers, err := b.compileAggAssignments(a.Aggs)
	if err != nil {
		return nil, err
	}
	dir := order.Direction(a.InputSortDir)
	return aggregate.New(b.rctx, parent, keys, names, reducers, a.Limit, dir, a.PartialsIn, a.PartialsOut, b.resetters)
}

func (b *Builder) compileAggAssignments(assignments []dag.Assignment) (field.List, []*expr.Aggregator, error) {
	names := make(field.List, 0, len(assignments))
	aggs := make([]*expr.Aggregator, 0, len(assignments))
	for _, assignment := range assignments {
		name, agg, err := b.compileAggAssignment(assignment)
		if err != nil {
			return nil, nil, err
		}
		aggs = append(aggs, agg)
		names = append(names, name)
	}
	return names, aggs, nil
}

func (b *Builder) compileAggAssignment(assignment dag.Assignment) (field.Path, *expr.Aggregator, error) {
	aggAST, ok := assignment.RHS.(*dag.Agg)
	if !ok {
		return nil, nil, errors.New("aggregator is not an aggregation expression")
	}
	this, ok := assignment.LHS.(*dag.This)
	if !ok {
		return nil, nil, fmt.Errorf("internal error: aggregator assignment LHS is not a static path: %#v", assignment.LHS)
	}
	m, err := b.compileAgg(aggAST)
	return this.Path, m, err
}

func (b *Builder) compileAgg(agg *dag.Agg) (*expr.Aggregator, error) {
	name := agg.Name
	var err error
	var arg expr.Evaluator
	if agg.Expr != nil {
		arg, err = b.compileExpr(agg.Expr)
		if err != nil {
			return nil, err
		}
	}
	var where expr.Evaluator
	if agg.Where != nil {
		where, err = b.compileExpr(agg.Where)
		if err != nil {
			return nil, err
		}
	}
	return expr.NewAggregator(name, arg, where)
}
