// Package ast declares the types used to represent syntax trees for Zed
// queries.
package ast

import (
	astzed "github.com/brimdata/zed/compiler/ast/zed"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/field"
)

// This module is derived from the GO AST design pattern in
// https://golang.org/pkg/go/ast/
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Proc is the interface implemented by all AST processor nodes.
type Proc interface {
	ProcAST()
}

type Expr interface {
	ExprAST()
}

type ID struct {
	Kind string `json:"kind" unpack:""`
	Name string `json:"name"`
}

type Term struct {
	Kind  string           `json:"kind" unpack:""`
	Text  string           `json:"text"`
	Value astzed.Primitive `json:"value"`
}

type UnaryExpr struct {
	Kind    string `json:"kind" unpack:""`
	Op      string `json:"op"`
	Operand Expr   `json:"operand"`
}

// A BinaryExpr is any expression of the form "lhs kind rhs"
// including arithmetic (+, -, *, /), logical operators (and, or),
// comparisons (=, !=, <, <=, >, >=), index operatons (on arrays, sets, and records)
// with kind "[" and a dot expression (".") (on records).
type BinaryExpr struct {
	Kind string `json:"kind" unpack:""`
	Op   string `json:"op"`
	LHS  Expr   `json:"lhs"`
	RHS  Expr   `json:"rhs"`
}

type Conditional struct {
	Kind string `json:"kind" unpack:""`
	Cond Expr   `json:"cond"`
	Then Expr   `json:"then"`
	Else Expr   `json:"else"`
}

// A Call represents different things dependending on its context.
// As a proc, it is either a group-by with no group-by keys and no duration,
// or a filter with a function that is boolean valued.  This is determined
// by the compiler rather than the syntax tree based on the specific functions
// and aggregators that are defined at compile time.  In expression context,
// a function call has the standard semantics where it takes one or more arguments
// and returns a result.
type Call struct {
	Kind  string `json:"kind" unpack:""`
	Name  string `json:"name"`
	Args  []Expr `json:"args"`
	Where Expr   `json:"where"`
}

type Cast struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Type Expr   `json:"type"`
}

type Grep struct {
	Kind    string  `json:"kind" unpack:""`
	Pattern Pattern `json:"pattern"`
	Expr    Expr    `json:"expr"`
}

type Glob struct {
	Kind    string `json:"kind" unpack:""`
	Pattern string `json:"pattern"`
}

type Regexp struct {
	Kind    string `json:"kind" unpack:""`
	Pattern string `json:"pattern"`
}

type String struct {
	Kind string `json:"kind" unpack:""`
	Text string `json:"text"`
}

type Pattern interface {
	PatternAST()
}

func (*Glob) PatternAST()   {}
func (*Regexp) PatternAST() {}
func (*String) PatternAST() {}

type RecordExpr struct {
	Kind  string       `json:"kind" unpack:""`
	Elems []RecordElem `json:"elems"`
}

type RecordElem interface {
	recordAST()
}

func (*Field) recordAST()  {}
func (*ID) recordAST()     {}
func (*Spread) recordAST() {}

type Field struct {
	Kind  string `json:"kind" unpack:""`
	Name  string `json:"name"`
	Value Expr   `json:"value"`
}

type Spread struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
}

type ArrayExpr struct {
	Kind  string `json:"kind" unpack:""`
	Exprs []Expr `json:"exprs"`
}

type SetExpr struct {
	Kind  string `json:"kind" unpack:""`
	Exprs []Expr `json:"exprs"`
}

type MapExpr struct {
	Kind    string      `json:"kind" unpack:""`
	Entries []EntryExpr `json:"entries"`
}

type EntryExpr struct {
	Key   Expr `json:"key"`
	Value Expr `json:"value"`
}

func (*UnaryExpr) ExprAST()   {}
func (*BinaryExpr) ExprAST()  {}
func (*Conditional) ExprAST() {}
func (*Call) ExprAST()        {}
func (*Cast) ExprAST()        {}
func (*ID) ExprAST()          {}

func (*Assignment) ExprAST() {}
func (*Agg) ExprAST()        {}
func (*Grep) ExprAST()       {}
func (*Glob) ExprAST()       {}
func (*Regexp) ExprAST()     {}
func (*Term) ExprAST()       {}

func (*RecordExpr) ExprAST() {}
func (*ArrayExpr) ExprAST()  {}
func (*SetExpr) ExprAST()    {}
func (*MapExpr) ExprAST()    {}

func (*SQLExpr) ExprAST() {}

// ----------------------------------------------------------------------------
// Procs

// A proc is a node in the flowgraph that takes records in, processes them,
// and produces records as output.
//
type (
	// A Sequential proc represents a set of procs that receive
	// a stream of records from their parent into the first proc
	// and each subsequent proc processes the output records from the
	// previous proc.
	Sequential struct {
		Kind   string `json:"kind" unpack:""`
		Consts []Def  `json:"consts"`
		Procs  []Proc `json:"procs"`
	}
	// A Parallel proc represents a set of procs that each get
	// a stream of records from their parent.
	Parallel struct {
		Kind string `json:"kind" unpack:""`
		// If non-zero, MergeBy contains the field name on
		// which the branches of this parallel proc should be
		// merged in the order indicated by MergeReverse.
		// XXX merge_by should be a list of expressions
		MergeBy      field.Path `json:"merge_by,omitempty"`
		MergeReverse bool       `json:"merge_reverse,omitempty"`
		Procs        []Proc     `json:"procs"`
	}
	// A Switch proc represents a set of procs that each get
	// a stream of records from their parent.
	Switch struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Cases []Case `json:"cases"`
	}
	// A Sort proc represents a proc that sorts records.
	Sort struct {
		Kind       string      `json:"kind" unpack:""`
		Args       []Expr      `json:"args"`
		Order      order.Which `json:"order"`
		NullsFirst bool        `json:"nullsfirst"`
	}
	// A Cut proc represents a proc that removes fields from each
	// input record where each removed field matches one of the named fields
	// sending each such modified record to its output in the order received.
	Cut struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	// A Drop proc represents a proc that removes fields from each
	// input record.
	Drop struct {
		Kind string `json:"kind" unpack:""`
		Args []Expr `json:"args"`
	}
	// A Head proc represents a proc that forwards the indicated number
	// of records then terminates.
	Head struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	// A Tail proc represents a proc that reads all its records from its
	// input transmits the final number of records indicated by the count.
	Tail struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	// A Pass proc represents a passthrough proc that mirrors
	// incoming Pull()s on its parent and returns the result.
	Pass struct {
		Kind string `json:"kind" unpack:""`
	}
	// A Uniq proc represents a proc that discards any record that matches
	// the previous record transmitted.  The Cflag causes the output records
	// to contain a new field called count that contains the number of matched
	// records in that set, similar to the unix shell command uniq.
	Uniq struct {
		Kind  string `json:"kind" unpack:""`
		Cflag bool   `json:"cflag"`
	}
	// A Summarize proc represents a proc that consumes all the records
	// in its input, partitions the records into groups based on the values
	// of the fields specified in the keys field (where the first key is the
	// primary grouping key), and applies aggregators (if any) to each group.
	// The InputSortDir field indicates that input is sorted (with
	// direction indicated by the sign of the field) in the primary
	// grouping key. In this case, the proc outputs the aggregation
	// results from each key as they complete so that large inputs
	// are processed and streamed efficiently.
	// The Limit field specifies the number of different groups that can be
	// aggregated over. When absent, the runtime defaults to an
	// appropriate value.
	// If PartialsOut is true, the proc will produce partial aggregation
	// output result; likewise, if PartialsIn is true, the proc will
	// expect partial results as input.
	Summarize struct {
		Kind  string       `json:"kind" unpack:""`
		Limit int          `json:"limit"`
		Keys  []Assignment `json:"keys"`
		Aggs  []Assignment `json:"aggs"`
	}
	// A Top proc is similar to a Sort with a few key differences:
	// - It only sorts in descending order.
	// - It utilizes a MaxHeap, immediately discarding records that are not in
	// the top N of the sort.
	// - It has an option (Flush) to sort and emit on every batch.
	Top struct {
		Kind  string `json:"kind" unpack:""`
		Limit int    `json:"limit"`
		Args  []Expr `json:"args"`
		Flush bool   `json:"flush"`
	}
	Put struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Merge struct {
		Kind  string `json:"kind" unpack:""`
		Field Expr   `json:"field"`
	}
	Over struct {
		Kind  string      `json:"kind" unpack:""`
		Exprs []Expr      `json:"exprs"`
		As    string      `json:"as"`
		Scope *Sequential `json:"scope"`
	}
	Let struct {
		Kind   string `json:"kind" unpack:""`
		Locals []Def  `json:"locals"`
		Over   *Over  `json:"over"`
	}
	Search struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Where struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Yield struct {
		Kind  string `json:"kind" unpack:""`
		Exprs []Expr `json:"exprs"`
	}

	// An OpAssignment proc is a list of assignments whose parent proc
	// is unknown: It could be a Summarize or Put proc. This will be
	// determined in the semantic phase.
	OpAssignment struct {
		Kind        string       `json:"kind" unpack:""`
		Assignments []Assignment `json:"assignments"`
	}
	// An OpExpr operator is an expression that appears as an operator
	// and requires semantic analysis to determine if it is a filter, a yield,
	// or an aggregation.
	OpExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}

	// A Rename proc represents a proc that renames fields.
	Rename struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}

	// A Fuse proc represents a proc that turns a zng stream into a dataframe.
	Fuse struct {
		Kind string `json:"kind" unpack:""`
	}

	// A Join proc represents a proc that joins two zng streams.
	Join struct {
		Kind     string       `json:"kind" unpack:""`
		Style    string       `json:"style"`
		LeftKey  Expr         `json:"left_key"`
		RightKey Expr         `json:"right_key"`
		Args     []Assignment `json:"args"`
	}

	// A SQLExpr can be a proc, an expression inside of a SQL FROM clause,
	// or an expression used as a Zed value generator.  Currenly, the "select"
	// keyword collides with the select() generator function (it can be parsed
	// unambiguosly because of the parens but this is not user friendly
	// so we need a new name for select()... see issue #2133).
	// TBD: from alias, "in" over tuples, WITH sub-queries, multi-table FROM
	// implying a JOIN, aliases for tables in FROM and JOIN.
	SQLExpr struct {
		Kind    string       `json:"kind" unpack:""`
		Select  []Assignment `json:"select"`
		From    *SQLFrom     `json:"from"`
		Joins   []SQLJoin    `json:"joins"`
		Where   Expr         `json:"where"`
		GroupBy []Expr       `json:"group_by"`
		Having  Expr         `json:"having"`
		OrderBy *SQLOrderBy  `json:"order_by"`
		Limit   int          `json:"limit"`
	}
	Shape struct {
		Kind string `json:"kind" unpack:""`
	}
	From struct {
		Kind   string  `json:"kind" unpack:""`
		Trunks []Trunk `json:"trunks"`
	}
)

// Source structure

type (
	File struct {
		Kind   string  `json:"kind" unpack:""`
		Path   string  `json:"path"`
		Format string  `json:"format"`
		Layout *Layout `json:"layout"`
	}
	HTTP struct {
		Kind   string  `json:"kind" unpack:""`
		URL    string  `json:"url"`
		Format string  `json:"format"`
		Layout *Layout `json:"layout"`
	}
	Pool struct {
		Kind      string   `json:"kind" unpack:""`
		Spec      PoolSpec `json:"spec"`
		At        string   `json:"at"`
		Range     *Range   `json:"range"`
		ScanOrder string   `json:"scan_order"` // asc, desc, or unknown
	}
	Explode struct {
		Kind string      `json:"kind" unpack:""`
		Args []Expr      `json:"args"`
		Type astzed.Type `json:"type"`
		As   Expr        `json:"as"`
	}
)

type PoolSpec struct {
	Pool   string `json:"pool"`
	Commit string `json:"commit"`
	Meta   string `json:"meta"`
}

type Range struct {
	Kind  string `json:"kind" unpack:""`
	Lower Expr   `json:"lower"`
	Upper Expr   `json:"upper"`
}

type Source interface {
	Source()
}

func (*Pool) Source() {}
func (*File) Source() {}
func (*HTTP) Source() {}
func (*Pass) Source() {}

type Layout struct {
	Kind  string `json:"kind" unpack:""`
	Keys  []Expr `json:"keys"`
	Order string `json:"order"`
}

type Trunk struct {
	Kind   string      `json:"kind" unpack:""`
	Source Source      `json:"source"`
	Seq    *Sequential `json:"seq"`
}

type Case struct {
	Expr Expr `json:"expr"`
	Proc Proc `json:"proc"`
}

type SQLFrom struct {
	Table Expr `json:"table"`
	Alias Expr `json:"alias"`
}

type SQLOrderBy struct {
	Kind  string      `json:"kind" unpack:""`
	Keys  []Expr      `json:"keys"`
	Order order.Which `json:"order"`
}

type SQLJoin struct {
	Table    Expr   `json:"table"`
	Style    string `json:"style"`
	LeftKey  Expr   `json:"left_key"`
	RightKey Expr   `json:"right_key"`
	Alias    Expr   `json:"alias"`
}

type Assignment struct {
	Kind string `json:"kind" unpack:""`
	LHS  Expr   `json:"lhs"`
	RHS  Expr   `json:"rhs"`
}

// Def is like Assignment but the LHS is an identifier that may be later
// referenced.  This is used for const blocks in Sequential and var blocks
// in a let scope.
type Def struct {
	Name string `json:"name"`
	Expr Expr   `json:"expr"`
}

func (*Sequential) ProcAST()   {}
func (*Parallel) ProcAST()     {}
func (*Switch) ProcAST()       {}
func (*Sort) ProcAST()         {}
func (*Cut) ProcAST()          {}
func (*Drop) ProcAST()         {}
func (*Head) ProcAST()         {}
func (*Tail) ProcAST()         {}
func (*Pass) ProcAST()         {}
func (*Uniq) ProcAST()         {}
func (*Summarize) ProcAST()    {}
func (*Top) ProcAST()          {}
func (*Put) ProcAST()          {}
func (*OpAssignment) ProcAST() {}
func (*OpExpr) ProcAST()       {}
func (*Rename) ProcAST()       {}
func (*Fuse) ProcAST()         {}
func (*Join) ProcAST()         {}
func (*Shape) ProcAST()        {}
func (*From) ProcAST()         {}
func (*Explode) ProcAST()      {}
func (*Merge) ProcAST()        {}
func (*Over) ProcAST()         {}
func (*Let) ProcAST()          {}
func (*Search) ProcAST()       {}
func (*Where) ProcAST()        {}
func (*Yield) ProcAST()        {}

func (*SQLExpr) ProcAST() {}

func (seq *Sequential) Prepend(front Proc) {
	seq.Procs = append([]Proc{front}, seq.Procs...)
}

// An Agg is an AST node that represents a aggregate function.  The Name
// field indicates the aggregation method while the Expr field indicates
// an expression applied to the incoming records that is operated upon by them
// aggregate function.  If Expr isn't present, then the aggregator doesn't act
// upon a function of the record, e.g., count() counts up records without
// looking into them.
type Agg struct {
	Kind  string `json:"kind" unpack:""`
	Name  string `json:"name"`
	Expr  Expr   `json:"expr"`
	Where Expr   `json:"where"`
}

func NewDotExpr(f field.Path) Expr {
	lhs := Expr(&ID{
		Kind: "ID",
		Name: "this",
	})
	for _, name := range f {
		rhs := &ID{
			Kind: "ID",
			Name: name,
		}
		lhs = &BinaryExpr{
			Kind: "BinaryExpr",
			Op:   ".",
			LHS:  lhs,
			RHS:  rhs,
		}
	}
	return lhs
}

func NewLayout(layout order.Layout) *Layout {
	var keys []Expr
	for _, key := range layout.Keys {
		keys = append(keys, NewDotExpr(key))
	}
	return &Layout{
		Keys:  keys,
		Order: layout.Order.String(),
	}
}

func NewAggAssignment(kind string, lval field.Path, arg field.Path) Assignment {
	agg := &Agg{Kind: "Agg", Name: kind}
	if arg != nil {
		agg.Expr = NewDotExpr(arg)
	}
	lhs := lval
	if lhs == nil {
		lhs = field.New(kind)
	}
	return Assignment{
		Kind: "Assignment",
		LHS:  NewDotExpr(lhs),
		RHS:  agg,
	}
}
