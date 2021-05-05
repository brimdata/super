package dag

// This module is derived from the GO AST design pattern in
// https://golang.org/pkg/go/ast/
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"github.com/brimdata/zed/compiler/ast/zed"
	"github.com/brimdata/zed/field"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/segmentio/ksuid"
)

type Op interface {
	OpNode()
}

var PassOp = &Pass{Kind: "Pass"}

// Ops

type (
	Cut struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Drop struct {
		Kind string `json:"kind" unpack:""`
		Args []Expr `json:"args"`
	}
	Filter struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Fuse struct {
		Kind string `json:"kind" unpack:""`
	}
	Head struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	Join struct {
		Kind     string       `json:"kind" unpack:""`
		Style    string       `json:"style"`
		LeftKey  Expr         `json:"left_key"`
		RightKey Expr         `json:"right_key"`
		Args     []Assignment `json:"args"`
	}
	Merge struct {
		Kind    string     `json:"kind" unpack:""`
		Key     field.Path `json:"key"`
		Reverse bool       `json:"reverse"`
	}
	Parallel struct {
		Kind string `json:"kind" unpack:""`
		Ops  []Op   `json:"ops"`
	}
	Pass struct {
		Kind string `json:"kind" unpack:""`
	}
	Pick struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Put struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Rename struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Sequential struct {
		Kind string `json:"kind" unpack:""`
		Ops  []Op   `json:"ops"`
	}
	Shape struct {
		Kind string `json:"kind" unpack:""`
	}
	Sort struct {
		Kind       string `json:"kind" unpack:""`
		Args       []Expr `json:"args"`
		SortDir    int    `json:"sortdir"`
		NullsFirst bool   `json:"nullsfirst"`
	}
	Summarize struct {
		Kind         string         `json:"kind" unpack:""`
		Duration     *zed.Primitive `json:"duration"`
		Limit        int            `json:"limit"`
		Keys         []Assignment   `json:"keys"`
		Aggs         []Assignment   `json:"aggs"`
		InputSortDir int            `json:"input_sort_dir,omitempty"`
		PartialsIn   bool           `json:"partials_in,omitempty"`
		PartialsOut  bool           `json:"partials_out,omitempty"`
	}
	Switch struct {
		Kind  string `json:"kind" unpack:""`
		Cases []Case `json:"cases"`
	}
	Tail struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	Top struct {
		Kind  string `json:"kind" unpack:""`
		Limit int    `json:"limit"`
		Args  []Expr `json:"args"`
		Flush bool   `json:"flush"`
	}
	Uniq struct {
		Kind  string `json:"kind" unpack:""`
		Cflag bool   `json:"cflag"`
	}
)

// Input structure

type (
	From struct {
		Kind   string  `json:"kind" unpack:""`
		Trunks []Trunk `json:"trunks"`
	}

	// A Trunk is the path into a DAG for any input source.  It contains
	// the source to scan as well as the sequential operators to apply
	// to the scan before being joined, merged, or output.  A DAG can be
	// just one Trunk or an assembly of different Trunks mixed in using
	// the From Op.  The Trunk is the one place where the optimizer places
	// pushed down predicates so the runtime can move the pushed down
	// operators into each scan scheduler for each source when the runtime
	// is built.  When computation is distribtued over the network, the
	// optimized pushdown is naturally carried in the serialized DAG via
	// each Trunk.
	Trunk struct {
		Kind     string      `json:"kind" unpack:""`
		Source   Source      `json:"source"`
		Seq      *Sequential `json:"seq"`
		Pushdown Op          `json:"pushdown"`
	}

	// Leaf sources

	File struct {
		Kind   string       `json:"kind" unpack:""`
		Path   string       `json:"path"`
		Format string       `json:"format"`
		Layout order.Layout `json:"layout"`
	}
	HTTP struct {
		Kind   string       `json:"kind" unpack:""`
		URL    string       `json:"url"`
		Format string       `json:"format"`
		Layout order.Layout `json:"layout"`
	}
	Pool struct {
		Kind string      `json:"kind" unpack:""`
		ID   ksuid.KSUID `json:"id"`
		At   ksuid.KSUID `json:"at"`
		// Span needs to be replaced with Upper/Lower.  See #2482.
		//Upper  zed.Any `json:"upper"`
		//Lower    zed.Any `json:"lower"`
		Span      nano.Span `json:"span"`
		ScanOrder string    `json:"scan_order"`
		Group     int       `json:"group"`
	}
)

type Source interface {
	Source()
}

func (*File) Source() {}
func (*HTTP) Source() {}
func (*Pool) Source() {}
func (*Pass) Source() {}

// A From node can be a DAG entrypoint or an operator.  When it appears
// as an operator it mixes its single parent in with other Trunks to
// form a parallel structure whose output must be joined or merged.

func (*From) OpNode() {}

// Various Op fields

type (
	Assignment struct {
		Kind string `json:"kind" unpack:""`
		LHS  Expr   `json:"lhs"`
		RHS  Expr   `json:"rhs"`
	}
	Agg struct {
		Kind  string `json:"kind" unpack:""`
		Name  string `json:"name"`
		Expr  Expr   `json:"expr"`
		Where Expr   `json:"where"`
	}
	Case struct {
		Expr Expr `json:"expr"`
		Op   Op   `json:"op"`
	}
	Method struct {
		Name string `json:"name"`
		Args []Expr `json:"args"`
	}
)

func (*Sequential) OpNode()   {}
func (*Parallel) OpNode()     {}
func (*Switch) OpNode()       {}
func (*Sort) OpNode()         {}
func (*Cut) OpNode()          {}
func (*Pick) OpNode()         {}
func (*Drop) OpNode()         {}
func (*Head) OpNode()         {}
func (*Tail) OpNode()         {}
func (*Pass) OpNode()         {}
func (*Filter) OpNode()       {}
func (*Uniq) OpNode()         {}
func (*Summarize) OpNode()    {}
func (*Top) OpNode()          {}
func (*Put) OpNode()          {}
func (*Rename) OpNode()       {}
func (*Fuse) OpNode()         {}
func (*Join) OpNode()         {}
func (*Const) OpNode()        {}
func (*TypeProc) OpNode()     {}
func (*Shape) OpNode()        {}
func (*FieldCutter) OpNode()  {}
func (*TypeSplitter) OpNode() {}
func (*Merge) OpNode()        {}

func (seq *Sequential) IsEntry() bool {
	if len(seq.Ops) == 0 {
		return false
	}
	_, ok := seq.Ops[0].(*From)
	return ok
}

func (seq *Sequential) Prepend(front Op) {
	seq.Ops = append([]Op{front}, seq.Ops...)
}

func (seq *Sequential) Append(op Op) {
	seq.Ops = append(seq.Ops, op)
}

func (seq *Sequential) Delete(at, length int) {
	seq.Ops = append(seq.Ops[0:at], seq.Ops[at+length:]...)
}

func FanIn(op Op) int {
	switch op := op.(type) {
	case *Sequential:
		return FanIn(op.Ops[0])
	case *Join:
		return 2
	}
	return 1
}

func FilterToOp(e Expr) *Filter {
	return &Filter{
		Kind: "Filter",
		Expr: e,
	}
}

func (p *Path) String() string {
	return field.Path(p.Name).String()
}

// === THESE SHOULD BE RENAMED AND MADE PART OF THE LANGUAGE ===

type FieldCutter struct {
	Kind  string     `json:"kind" unpack:""`
	Field field.Path `json:"field"`
	Out   field.Path `json:"out"`
}

type TypeSplitter struct {
	Kind     string     `json:"kind" unpack:""`
	Key      field.Path `json:"key"`
	TypeName string     `json:"type_name"`
}

// === THESE WILL BE DEPRECATED ===

type Const struct {
	Kind string `json:"kind" unpack:""`
	Name string `json:"name"`
	Expr Expr   `json:"expr"`
}
type TypeProc struct {
	Kind string   `json:"kind" unpack:""`
	Name string   `json:"name"`
	Type zed.Type `json:"type"`
}
