package describe

import (
	"context"
	"fmt"

	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/semantic"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/segmentio/ksuid"
)

type Info struct {
	Sources  []Source  `json:"sources"`
	Channels []Channel `json:"channels"`
}

type Source interface {
	Source()
}

type (
	LakeMeta struct {
		Kind string `json:"kind"`
		Meta string `json:"meta"`
	}
	Pool struct {
		Kind     string      `json:"kind"`
		Name     string      `json:"name"`
		ID       ksuid.KSUID `json:"id"`
		Inferred bool        `json:"inferred"`
	}
	Path struct {
		Kind     string `json:"kind"`
		URI      string `json:"uri"`
		Inferred bool   `json:"inferred"`
	}
)

func (*LakeMeta) Source() {}
func (*Pool) Source()     {}
func (*Path) Source()     {}

type Channel struct {
	Name            string         `json:"name"`
	AggregationKeys field.List     `json:"aggregation_keys"`
	Sort            order.SortKeys `json:"sort"`
}

func Analyze(ctx context.Context, sql, bool, query string, src *data.Source, head *lakeparse.Commitish) (*Info, error) {
	seq, sset, err := parser.ParseSuperPipe(sql, nil, query)
	if err != nil {
		return nil, err
	}
	entry, err := semantic.Analyze(ctx, seq, src, head)
	if err != nil {
		if list, ok := err.(parser.ErrorList); ok {
			list.SetSourceSet(sset)
		}
		return nil, err
	}
	return AnalyzeDAG(ctx, entry, src, head)
}

func AnalyzeDAG(ctx context.Context, entry dag.Seq, src *data.Source, head *lakeparse.Commitish) (*Info, error) {
	srcInferred := !semantic.HasSource(entry)
	if err := semantic.AddDefaultSource(ctx, &entry, src, head); err != nil {
		return nil, err
	}
	var err error
	var info Info
	if info.Sources, err = describeSources(ctx, src.Lake(), entry[0], srcInferred); err != nil {
		return nil, err
	}
	sortKeys, err := optimizer.New(ctx, src).SortKeys(entry)
	if err != nil {
		return nil, err
	}
	aggKeys := describeAggs(entry, []field.List{nil})
	outputs := collectOutputs(entry)
	m := make(map[string]int)
	for i, s := range sortKeys {
		name := outputs[i].Name
		if k, ok := m[name]; ok {
			// If output already exists, this means the outputs will be
			// combined so nil everything out.
			// XXX This is currently what happens but is this right?
			c := &info.Channels[k]
			c.Sort, c.AggregationKeys = nil, nil
			continue
		}
		info.Channels = append(info.Channels, Channel{
			Name:            name,
			Sort:            s,
			AggregationKeys: aggKeys[i],
		})
		m[name] = i
	}
	return &info, nil
}

func describeSources(ctx context.Context, lk *lake.Root, o dag.Op, inferred bool) ([]Source, error) {
	switch o := o.(type) {
	case *dag.Scope:
		return describeSources(ctx, lk, o.Body[0], inferred)
	case *dag.Fork:
		var s []Source
		for _, p := range o.Paths {
			out, err := describeSources(ctx, lk, p[0], inferred)
			if err != nil {
				return nil, err
			}
			s = append(s, out...)
		}
		return s, nil
	case *dag.DefaultScan:
		return []Source{&Path{Kind: "Path", URI: "stdio://stdin", Inferred: inferred}}, nil
	case *dag.FileScan:
		return []Source{&Path{Kind: "Path", URI: o.Path, Inferred: inferred}}, nil
	case *dag.HTTPScan:
		return []Source{&Path{Kind: "Path", URI: o.URL, Inferred: inferred}}, nil
	case *dag.PoolScan:
		return sourceOfPool(ctx, lk, o.ID, inferred)
	case *dag.Lister:
		return sourceOfPool(ctx, lk, o.Pool, inferred)
	case *dag.SeqScan:
		return sourceOfPool(ctx, lk, o.Pool, inferred)
	case *dag.CommitMetaScan:
		return sourceOfPool(ctx, lk, o.Pool, inferred)
	case *dag.LakeMetaScan:
		return []Source{&LakeMeta{Kind: "LakeMeta", Meta: o.Meta}}, nil
	default:
		return nil, fmt.Errorf("unsupported source type %T", o)
	}
}

func sourceOfPool(ctx context.Context, lk *lake.Root, id ksuid.KSUID, inferred bool) ([]Source, error) {
	p, err := lk.OpenPool(ctx, id)
	if err != nil {
		return nil, err
	}
	return []Source{&Pool{
		Kind:     "Pool",
		ID:       id,
		Name:     p.Name,
		Inferred: inferred,
	}}, nil
}

func describeAggs(seq dag.Seq, parents []field.List) []field.List {
	for _, op := range seq {
		parents = describeOpAggs(op, parents)
	}
	return parents
}

func describeOpAggs(op dag.Op, parents []field.List) []field.List {
	switch op := op.(type) {
	case *dag.Scope:
		return describeAggs(op.Body, parents)
	case *dag.Fork:
		var aggs []field.List
		for _, p := range op.Paths {
			aggs = append(aggs, describeAggs(p, []field.List{nil})...)
		}
		return aggs
	case *dag.Scatter:
		var aggs []field.List
		for _, p := range op.Paths {
			aggs = append(aggs, describeAggs(p, []field.List{nil})...)
		}
		return aggs
	case *dag.Mirror:
		aggs := describeAggs(op.Main, []field.List{nil})
		return append(aggs, describeAggs(op.Mirror, []field.List{nil})...)
	case *dag.Summarize:
		// The field list for aggregation with no keys is an empty slice and
		// not nil.
		keys := field.List{}
		for _, k := range op.Keys {
			keys = append(keys, k.LHS.(*dag.This).Path)
		}
		return []field.List{keys}
	}
	// If more than one parent reset to nil aggregation.
	if len(parents) > 1 {
		return []field.List{nil}
	}
	return parents
}

func collectOutputs(seq dag.Seq) []*dag.Output {
	var outputs []*dag.Output
	optimizer.Walk(seq, func(seq dag.Seq) dag.Seq {
		if len(seq) > 0 {
			if o, ok := seq[len(seq)-1].(*dag.Output); ok {
				outputs = append(outputs, o)
			}
		}
		return seq
	})
	return outputs
}
