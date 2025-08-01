// Package ast declares the types used to represent syntax trees for Zed
// queries.
package ast

// This module is derived from the GO AST design pattern in
// https://golang.org/pkg/go/ast/
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

type Node interface {
	Pos() int // Position of first character belonging to the node.
	End() int // Position of first character immediately after the node.
}

type Loc struct {
	First int `json:"first"`
	Last  int `json:"last"`
}

func NewLoc(pos, end int) Loc {
	return Loc{pos, end}
}

func (l Loc) Pos() int { return l.First }
func (l Loc) End() int { return l.Last }

// Op is the interface implemented by all AST operator nodes.
type Op interface {
	Node
	opNode()
}

type Decl interface {
	Node
	declNode()
}

type Expr interface {
	Node
	exprNode()
}

type ID struct {
	Kind string `json:"kind" unpack:""`
	Name string `json:"name"`
	Loc  `json:"loc"`
}

// DoubleQuote is specialized from the other primitive types because
// these values can be interpreted either as a string value or an identifier based
// on SQL vs pipe context.  The semantic pass needs to know the string was
// originally double quoted to perform this analysis.
type DoubleQuote struct {
	Kind string `json:"kind" unpack:""`
	Text string `json:"text"`
	Loc  `json:"loc"`
}

type Term struct {
	Kind  string `json:"kind" unpack:""`
	Text  string `json:"text"`
	Value Any    `json:"value"`
	Loc   `json:"loc"`
}

type UnaryExpr struct {
	Kind    string `json:"kind" unpack:""`
	Op      string `json:"op"`
	Operand Expr   `json:"operand"`
	Loc     `json:"loc"`
}

// A BinaryExpr is any expression of the form "lhs kind rhs"
// including arithmetic (+, -, *, /), logical operators (and, or),
// comparisons (=, !=, <, <=, >, >=), and a dot expression (".") (on records).
type BinaryExpr struct {
	Kind string `json:"kind" unpack:""`
	Op   string `json:"op"`
	LHS  Expr   `json:"lhs"`
	RHS  Expr   `json:"rhs"`
	Loc  `json:"loc"`
}

type Between struct {
	Kind  string `json:"kind" unpack:""`
	Not   bool   `json:"not"`
	Expr  Expr   `json:"expr"`
	Lower Expr   `json:"lower"`
	Upper Expr   `json:"upper"`
	Loc   `json:"loc"`
}

type Conditional struct {
	Kind string `json:"kind" unpack:""`
	Cond Expr   `json:"cond"`
	Then Expr   `json:"then"`
	Else Expr   `json:"else"`
	Loc  `json:"loc"`
}

type CaseExpr struct {
	Kind  string `json:"kind" unpack:""`
	Expr  Expr   `json:"expr"`
	Whens []When `json:"whens"`
	Else  Expr   `json:"else"`
	Loc   `json:"loc"`
}

type When struct {
	Kind string `json:"kind" unpack:""`
	Cond Expr   `json:"expr"`
	Then Expr   `json:"else"`
	Loc  `json:"loc"`
}

// A Call represents different things dependending on its context.
// As an operator, it is either an aggregate with no grouping keys and no duration,
// or a filter with a function that is boolean valued.  This is determined
// by the compiler rather than the syntax tree based on the specific functions
// and aggregators that are defined at compile time.  In expression context,
// a function call has the standard semantics where it takes one or more arguments
// and returns a result.
type Call struct {
	Kind  string `json:"kind" unpack:""`
	Name  *ID    `json:"name"`
	Args  []Expr `json:"args"`
	Where Expr   `json:"where"`
	Loc   `json:"loc"`
}

type CallExtract struct {
	Kind string `json:"kind" unpack:""`
	Part Expr   `json:"part"`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

type Cast struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Type Expr   `json:"type"`
	Loc  `json:"loc"`
}

type IndexExpr struct {
	Kind  string `json:"kind" unpack:""`
	Expr  Expr   `json:"expr"`
	Index Expr   `json:"index"`
	Loc   `json:"loc"`
}

type IsNullExpr struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Not  bool   `json:"not"`
	Loc  `json:"loc"`
}

type SliceExpr struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	From Expr   `json:"from"`
	To   Expr   `json:"to"`
	Loc  `json:"loc"`
}

type Grep struct {
	Kind    string `json:"kind" unpack:""`
	Pattern Expr   `json:"pattern"`
	Expr    Expr   `json:"expr"`
	Loc     `json:"loc"`
}

type Glob struct {
	Kind    string `json:"kind" unpack:""`
	Pattern string `json:"pattern"`
	Loc     `json:"loc"`
}

type Regexp struct {
	Kind       string `json:"kind" unpack:""`
	Pattern    string `json:"pattern"`
	PatternPos int    `json:"pattern_pos"`
	Loc        `json:"loc"`
}

type Text struct {
	Kind string `json:"kind" unpack:""`
	Text string `json:"value"`
	Loc  `json:"loc"`
}

type FromEntity interface {
	Node
	fromEntityNode()
}

type ExprEntity struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*Glob) fromEntityNode()         {}
func (*Regexp) fromEntityNode()       {}
func (*ExprEntity) fromEntityNode()   {}
func (*LakeMeta) fromEntityNode()     {}
func (*Text) fromEntityNode()         {}
func (*SQLCrossJoin) fromEntityNode() {}
func (*SQLJoin) fromEntityNode()      {}
func (*SQLPipe) fromEntityNode()      {}

type FromElem struct {
	Kind       string      `json:"kind" unpack:""`
	Entity     FromEntity  `json:"entity"`
	Args       []OpArg     `json:"args"`
	Ordinality *Ordinality `json:"ordinality"`
	Alias      *TableAlias `json:"alias"`
	Loc        `json:"loc"`
}

type Ordinality struct {
	Kind string `json:"kind" unpack:""`
	Loc  `json:"loc"`
}

type TableAlias struct {
	Kind    string `json:"kind" unpack:""`
	Name    string `json:"name"`
	Columns []*ID  `json:"columns"`
	Loc     `json:"loc"`
}

type RecordExpr struct {
	Kind  string       `json:"kind" unpack:""`
	Elems []RecordElem `json:"elems"`
	Loc   `json:"loc"`
}

type RecordElem interface {
	Node
	recordElemNode()
}

type FieldExpr struct {
	Kind  string `json:"kind" unpack:""`
	Name  *Text  `json:"name"`
	Value Expr   `json:"value"`
	Loc   `json:"loc"`
}

type Spread struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*FieldExpr) recordElemNode() {}
func (*ID) recordElemNode()        {}
func (*Spread) recordElemNode()    {}

type ArrayExpr struct {
	Kind  string       `json:"kind" unpack:""`
	Elems []VectorElem `json:"elems"`
	Loc   `json:"loc"`
}

type SetExpr struct {
	Kind  string       `json:"kind" unpack:""`
	Elems []VectorElem `json:"elems"`
	Loc   `json:"loc"`
}

type VectorElem interface {
	vectorElemNode()
}

func (*Spread) vectorElemNode()      {}
func (*VectorValue) vectorElemNode() {}

type VectorValue struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

type MapExpr struct {
	Kind    string      `json:"kind" unpack:""`
	Entries []EntryExpr `json:"entries"`
	Loc     `json:"loc"`
}

type EntryExpr struct {
	Key   Expr `json:"key"`
	Value Expr `json:"value"`
	Loc   `json:"loc"`
}

type TupleExpr struct {
	Kind  string `json:"kind" unpack:""`
	Elems []Expr `json:"elems"`
	Loc   `json:"loc"`
}

type UnnestExpr struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Body Seq    `json:"body"`
	Loc  `json:"loc"`
}

type QueryExpr struct {
	Kind string `json:"kind" unpack:""`
	Body Seq    `json:"body"`
	Loc  `json:"loc"`
}

type FString struct {
	Kind  string        `json:"kind" unpack:""`
	Elems []FStringElem `json:"elems"`
	Loc   `json:"loc"`
}

type FStringElem interface {
	Node
	fStringElemNode()
}

type FStringText struct {
	Kind string `json:"kind" unpack:""`
	Text string `json:"text"`
	Loc  `json:"loc"`
}

type FStringExpr struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

type SQLTimeValue struct {
	Kind  string     `json:"kind" unpack:""`
	Type  string     `json:"type"`
	Value *Primitive `json:"value"`
	Loc   `json:"loc"`
}

func (*FStringText) fStringElemNode() {}
func (*FStringExpr) fStringElemNode() {}

func (*UnaryExpr) exprNode()   {}
func (*BinaryExpr) exprNode()  {}
func (*Between) exprNode()     {}
func (*Conditional) exprNode() {}
func (*Call) exprNode()        {}
func (*CallExtract) exprNode() {}
func (*CaseExpr) exprNode()    {}
func (*Cast) exprNode()        {}
func (*DoubleQuote) exprNode() {}
func (*ID) exprNode()          {}
func (*IndexExpr) exprNode()   {}
func (*IsNullExpr) exprNode()  {}
func (*SliceExpr) exprNode()   {}

func (*Assignment) exprNode() {}
func (*Agg) exprNode()        {}
func (*Grep) exprNode()       {}
func (*Glob) exprNode()       {}
func (*Regexp) exprNode()     {}
func (*Term) exprNode()       {}

func (*RecordExpr) exprNode()   {}
func (*ArrayExpr) exprNode()    {}
func (*SetExpr) exprNode()      {}
func (*MapExpr) exprNode()      {}
func (*TupleExpr) exprNode()    {}
func (*SQLTimeValue) exprNode() {}
func (*UnnestExpr) exprNode()   {}
func (*FString) exprNode()      {}
func (*Primitive) exprNode()    {}
func (*TypeValue) exprNode()    {}
func (*QueryExpr) exprNode()    {}

type ConstDecl struct {
	Kind string `json:"kind" unpack:""`
	Name *ID    `json:"name"`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

type FuncDecl struct {
	Kind   string `json:"kind" unpack:""`
	Name   *ID    `json:"name"`
	Params []*ID  `json:"params"`
	Expr   Expr   `json:"expr"`
	Loc    `json:"loc"`
}

type OpDecl struct {
	Kind   string `json:"kind" unpack:""`
	Name   *ID    `json:"name"`
	Params []*ID  `json:"params"`
	Body   Seq    `json:"body"`
	Loc    `json:"loc"`
}

type TypeDecl struct {
	Kind string `json:"kind" unpack:""`
	Name *ID    `json:"name"`
	Type Type   `json:"type"`
	Loc  `json:"loc"`
}

func (*ConstDecl) declNode() {}
func (*FuncDecl) declNode()  {}
func (*OpDecl) declNode()    {}
func (*TypeDecl) declNode()  {}

// ----------------------------------------------------------------------------
// Operators

// A Seq represents a sequence of operators that receive
// a stream of Zed values from their parent into the first operator
// and each subsequent operator processes the output records from the
// previous operator.
type Seq []Op

func (s Seq) Pos() int {
	if len(s) == 0 {
		return -1
	}
	return s[0].Pos()
}

func (s Seq) End() int {
	if len(s) == 0 {
		return -1
	}
	return s[len(s)-1].End()
}

func (s *Seq) Prepend(front Op) {
	*s = append([]Op{front}, *s...)
}

// An Op is a node in the flowgraph that takes Zed values in, operates upon them,
// and produces Zed values as output.
type (
	Scope struct {
		Kind  string `json:"kind" unpack:""`
		Decls []Decl `json:"decls"`
		Body  Seq    `json:"body"`
		Loc   `json:"loc"`
	}
	// A Parallel operator represents a set of operators that each get
	// a copy of the input from its parent.
	Parallel struct {
		Kind  string `json:"kind" unpack:""`
		Paths []Seq  `json:"paths"`
		Loc   `json:"loc"`
	}
	Switch struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Cases []Case `json:"cases"`
		Loc   `json:"loc"`
	}
	Sort struct {
		Kind    string     `json:"kind" unpack:""`
		Exprs   []SortExpr `json:"exprs"`
		Reverse bool       `json:"reverse"`
		Loc     `json:"loc"`
	}
	Cut struct {
		Kind string      `json:"kind" unpack:""`
		Args Assignments `json:"args"`
		Loc  `json:"loc"`
	}
	Drop struct {
		Kind string `json:"kind" unpack:""`
		Args []Expr `json:"args"`
		Loc  `json:"loc"`
	}
	Explode struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Args       []Expr `json:"args"`
		Type       Type   `json:"type"`
		As         Expr   `json:"as"`
		Loc        `json:"loc"`
	}
	Head struct {
		Kind  string `json:"kind" unpack:""`
		Count Expr   `json:"count"`
		Loc   `json:"loc"`
	}
	Tail struct {
		Kind  string `json:"kind" unpack:""`
		Count Expr   `json:"count"`
		Loc   `json:"loc"`
	}
	Skip struct {
		Kind  string `json:"kind" unpack:""`
		Count Expr   `json:"count"`
		Loc   `json:"loc"`
	}
	Pass struct {
		Kind string `json:"kind" unpack:""`
		Loc  `json:"loc"`
	}
	Uniq struct {
		Kind  string `json:"kind" unpack:""`
		Cflag bool   `json:"cflag"`
		Loc   `json:"loc"`
	}
	Aggregate struct {
		Kind  string      `json:"kind" unpack:""`
		Limit int         `json:"limit"`
		Keys  Assignments `json:"keys"`
		Aggs  Assignments `json:"aggs"`
		Loc   `json:"loc"`
	}
	Top struct {
		Kind    string     `json:"kind" unpack:""`
		Limit   Expr       `json:"limit"`
		Exprs   []SortExpr `json:"expr"`
		Reverse bool       `json:"reverse"`
		Loc     `json:"loc"`
	}
	Put struct {
		Kind string      `json:"kind" unpack:""`
		Args Assignments `json:"args"`
		Loc  `json:"loc"`
	}
	Merge struct {
		Kind  string     `json:"kind" unpack:""`
		Exprs []SortExpr `json:"exprs"`
		Loc   `json:"loc"`
	}
	Unnest struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Body Seq    `json:"body"`
		Loc  `json:"loc"`
	}
	Search struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Values struct {
		Kind  string `json:"kind" unpack:""`
		Exprs []Expr `json:"exprs"`
		Loc   `json:"loc"`
	}
	Where struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	// An OpAssignment is a list of assignments whose parent operator
	// is unknown: It could be a Aggregate or Put operator. This will be
	// determined in the semantic phase.
	OpAssignment struct {
		Kind        string      `json:"kind" unpack:""`
		Assignments Assignments `json:"assignments"`
		Loc         `json:"loc"`
	}
	// An OpExpr operator is an expression that appears as an operator
	// and requires semantic analysis to determine if it is a filter, a values
	// op, or an aggregation.
	OpExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Rename struct {
		Kind string      `json:"kind" unpack:""`
		Args Assignments `json:"args"`
		Loc  `json:"loc"`
	}
	Fuse struct {
		Kind string `json:"kind" unpack:""`
		Loc  `json:"loc"`
	}
	Join struct {
		Kind       string     `json:"kind" unpack:""`
		Style      string     `json:"style"`
		RightInput Seq        `json:"right_input"`
		Alias      *JoinAlias `json:"alias"`
		Cond       JoinCond   `json:"cond"`
		Loc        `json:"loc"`
	}
	JoinAlias struct {
		Left, Right *ID
		Loc         `json:"loc"`
	}
	Shapes struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Shape struct {
		Kind string `json:"kind" unpack:""`
		Loc  `json:"loc"`
	}
	Load struct {
		Kind string  `json:"kind" unpack:""`
		Pool *Text   `json:"pool"`
		Args []OpArg `json:"args"`
		Loc  `json:"loc"`
	}
	Assert struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Text string `json:"text"`
		Loc  `json:"loc"`
	}
	Output struct {
		Kind string `json:"kind" unpack:""`
		Name *ID    `json:"name"`
		Loc  `json:"loc"`
	}
	Debug struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Distinct struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
)

type (
	From struct {
		Kind  string      `json:"kind" unpack:""`
		Elems []*FromElem `json:"elems"`
		Loc   `json:"loc"`
	}
	LakeMeta struct {
		Kind    string `json:"kind" unpack:""`
		MetaPos int    `json:"meta_pos"`
		Meta    *Text  `json:"meta"`
		Loc     `json:"loc"`
	}
	DefaultScan struct {
		Kind string `json:"kind" unpack:""`
	}
	Delete struct {
		Kind   string       `json:"kind" unpack:""`
		Pool   string       `json:"pool"`
		Branch string       `json:"branch"`
		Loc    `json:"loc"` // dummy field, not needed except to implement Node
	}
)

func (d *DefaultScan) Pos() int { return -1 }
func (d *DefaultScan) End() int { return -1 }

type ArgExpr struct {
	Kind  string `json:"kind" unpack:""`
	Key   string `json:"key"`
	Value Expr   `json:"value"`
	Loc   `json:"loc"`
}

type ArgText struct {
	Kind  string `json:"kind" unpack:""`
	Key   string `json:"key"`
	Value *Text  `json:"value"`
	Loc   `json:"loc"`
}

type OpArg interface {
	Node
	opArgNode()
}

func (*ArgExpr) opArgNode() {}
func (*ArgText) opArgNode() {}

type SortExpr struct {
	Kind  string `json:"kind" unpack:""`
	Expr  Expr   `json:"expr"`
	Order *ID    `json:"order"`
	Nulls *ID    `json:"nulls"`
	Loc   `json:"loc"`
}

type Case struct {
	Expr Expr `json:"expr"`
	Path Seq  `json:"path"`
}

type Assignment struct {
	Kind string `json:"kind" unpack:""`
	LHS  Expr   `json:"lhs"`
	RHS  Expr   `json:"rhs"`
	Loc  `json:"loc"`
}

type Assignments []Assignment

func (a Assignments) Pos() int { return a[0].Pos() }
func (a Assignments) End() int { return a[len(a)-1].End() }

func (*Scope) opNode()        {}
func (*Parallel) opNode()     {}
func (*Switch) opNode()       {}
func (*Sort) opNode()         {}
func (*Cut) opNode()          {}
func (*Drop) opNode()         {}
func (*Head) opNode()         {}
func (*Tail) opNode()         {}
func (*Skip) opNode()         {}
func (*Pass) opNode()         {}
func (*Uniq) opNode()         {}
func (*Aggregate) opNode()    {}
func (*Top) opNode()          {}
func (*Put) opNode()          {}
func (*OpAssignment) opNode() {}
func (*OpExpr) opNode()       {}
func (*Rename) opNode()       {}
func (*Fuse) opNode()         {}
func (*Join) opNode()         {}
func (*Shape) opNode()        {}
func (*From) opNode()         {}
func (*DefaultScan) opNode()  {}
func (*Explode) opNode()      {}
func (*Merge) opNode()        {}
func (*Unnest) opNode()       {}
func (*Search) opNode()       {}
func (*Values) opNode()       {}
func (*Where) opNode()        {}
func (*Shapes) opNode()       {}
func (*Load) opNode()         {}
func (*Assert) opNode()       {}
func (*Output) opNode()       {}
func (*Debug) opNode()        {}
func (*Distinct) opNode()     {}
func (*Delete) opNode()       {}

// An Agg is an AST node that represents a aggregate function.  The Name
// field indicates the aggregation method while the Expr field indicates
// an expression applied to the incoming records that is operated upon by them
// aggregate function.  If Expr isn't present, then the aggregator doesn't act
// upon a function of the record, e.g., count() counts up records without
// looking into them.
type Agg struct {
	Kind     string `json:"kind" unpack:""`
	Name     string `json:"name"`
	Distinct bool   `json:"distinct"`
	Expr     Expr   `json:"expr"`
	Where    Expr   `json:"where"`
	Loc      `json:"loc"`
}
