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
	OpAST()
}

type Decl interface {
	Node
	DeclAST()
}

type Expr interface {
	Node
	ExprAST()
}

type ID struct {
	Kind string `json:"kind" unpack:""`
	Name string `json:"name"`
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

type Name struct {
	Kind string `json:"kind" unpack:""`
	Text string `json:"text"`
	Loc  `json:"loc"`
}

type FromEntity interface {
	Node
	fromEntity()
}

type ExprEntity struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*Glob) fromEntity()       {}
func (*Regexp) fromEntity()     {}
func (*ExprEntity) fromEntity() {}
func (*LakeMeta) fromEntity()   {}
func (*Name) fromEntity()       {}
func (*CrossJoin) fromEntity()  {}
func (*SQLJoin) fromEntity()    {}
func (*SQLPipe) fromEntity()    {}

type FromElem struct {
	Kind       string      `json:"kind" unpack:""`
	Entity     FromEntity  `json:"entity"`
	Args       FromArgs    `json:"args"`
	Ordinality *Ordinality `json:"ordinality"`
	Alias      *Name       `json:"alias"`
	Loc        `json:"loc"`
}

type Ordinality struct {
	Kind string `json:"kind" unpack:""`
	Loc  `json:"loc"`
}

type RecordExpr struct {
	Kind  string       `json:"kind" unpack:""`
	Elems []RecordElem `json:"elems"`
	Loc   `json:"loc"`
}

type RecordElem interface {
	Node
	recordAST()
}

type FieldExpr struct {
	Kind  string `json:"kind" unpack:""`
	Name  *Name  `json:"name"`
	Value Expr   `json:"value"`
	Loc   `json:"loc"`
}

type Spread struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*FieldExpr) recordAST() {}
func (*ID) recordAST()        {}
func (*Spread) recordAST()    {}

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
	vectorAST()
}

func (*Spread) vectorAST()      {}
func (*VectorValue) vectorAST() {}

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

type OverExpr struct {
	Kind   string `json:"kind" unpack:""`
	Locals []Def  `json:"locals"`
	Exprs  []Expr `json:"exprs"`
	Body   Seq    `json:"body"`
	Loc    `json:"loc"`
}

type FString struct {
	Kind  string        `json:"kind" unpack:""`
	Elems []FStringElem `json:"elems"`
	Loc   `json:"loc"`
}

type FStringElem interface {
	Node
	FStringElem()
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

func (*FStringText) FStringElem() {}
func (*FStringExpr) FStringElem() {}

func (*UnaryExpr) ExprAST()   {}
func (*BinaryExpr) ExprAST()  {}
func (*Between) ExprAST()     {}
func (*Conditional) ExprAST() {}
func (*Call) ExprAST()        {}
func (*CallExtract) ExprAST() {}
func (*CaseExpr) ExprAST()    {}
func (*Cast) ExprAST()        {}
func (*ID) ExprAST()          {}
func (*IndexExpr) ExprAST()   {}
func (*IsNullExpr) ExprAST()  {}
func (*SliceExpr) ExprAST()   {}

func (*Assignment) ExprAST() {}
func (*Agg) ExprAST()        {}
func (*Grep) ExprAST()       {}
func (*Glob) ExprAST()       {}
func (*Regexp) ExprAST()     {}
func (*Term) ExprAST()       {}

func (*RecordExpr) ExprAST()   {}
func (*ArrayExpr) ExprAST()    {}
func (*SetExpr) ExprAST()      {}
func (*MapExpr) ExprAST()      {}
func (*TupleExpr) ExprAST()    {}
func (*SQLTimeValue) ExprAST() {}
func (*OverExpr) ExprAST()     {}
func (*FString) ExprAST()      {}

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

func (*ConstDecl) DeclAST() {}
func (*FuncDecl) DeclAST()  {}
func (*OpDecl) DeclAST()    {}
func (*TypeDecl) DeclAST()  {}

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
		Kind       string     `json:"kind" unpack:""`
		Reverse    bool       `json:"reverse"`
		NullsFirst bool       `json:"nullsfirst"`
		Args       []SortExpr `json:"args"`
		Loc        `json:"loc"`
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
		Kind  string `json:"kind" unpack:""`
		Limit Expr   `json:"limit"`
		Args  []Expr `json:"args"`
		Flush bool   `json:"flush"`
		Loc   `json:"loc"`
	}
	Put struct {
		Kind string      `json:"kind" unpack:""`
		Args Assignments `json:"args"`
		Loc  `json:"loc"`
	}
	Merge struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Over struct {
		Kind   string `json:"kind" unpack:""`
		Exprs  []Expr `json:"exprs"`
		Locals []Def  `json:"locals"`
		Body   Seq    `json:"body"`
		Loc    `json:"loc"`
	}
	Search struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Where struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Yield struct {
		Kind  string `json:"kind" unpack:""`
		Exprs []Expr `json:"exprs"`
		Loc   `json:"loc"`
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
	// and requires semantic analysis to determine if it is a filter, a yield,
	// or an aggregation.
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
		Kind       string      `json:"kind" unpack:""`
		Style      string      `json:"style"`
		RightInput Seq         `json:"right_input"`
		Cond       JoinExpr    `json:"cond"`
		Args       Assignments `json:"args"`
		Loc        `json:"loc"`
	}
	Sample struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	Shape struct {
		Kind string `json:"kind" unpack:""`
		Loc  `json:"loc"`
	}
	Load struct {
		Kind    string `json:"kind" unpack:""`
		Pool    *Name  `json:"pool"`
		Branch  *Name  `json:"branch"`
		Author  *Name  `json:"author"`
		Message *Name  `json:"message"`
		Meta    *Name  `json:"meta"`
		Loc     `json:"loc"`
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
		Args  FromArgs    `json:"args"`
		Loc   `json:"loc"`
	}
	LakeMeta struct {
		Kind    string `json:"kind" unpack:""`
		MetaPos int    `json:"meta_pos"`
		Meta    *Name  `json:"meta"`
		Loc     `json:"loc"`
	}
	Delete struct {
		Kind   string       `json:"kind" unpack:""`
		Pool   string       `json:"pool"`
		Branch string       `json:"branch"`
		Loc    `json:"loc"` // dummy field, not needed except to implement Node
	}
)

type PoolArgs struct {
	Kind   string `json:"kind" unpack:""`
	Commit *Name  `json:"commit"`
	Meta   *Name  `json:"meta"`
	Tap    bool   `json:"tap"`
	Loc    `json:"loc"`
}

type FormatArg struct {
	Kind   string `json:"kind" unpack:""`
	Format *Name  `json:"format"`
	Loc    `json:"loc"`
}

type HTTPArgs struct {
	Kind    string      `json:"kind" unpack:""`
	Format  *Name       `json:"format"`
	Method  *Name       `json:"method"`
	Headers *RecordExpr `json:"headers"`
	Body    *Name       `json:"body"`
	Loc     `json:"loc"`
}

type FromArgs interface {
	Node
	fromArgs()
}

func (*PoolArgs) fromArgs()  {}
func (*FormatArg) fromArgs() {}
func (*HTTPArgs) fromArgs()  {}

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

// Def is like Assignment but the LHS is an identifier that may be later
// referenced.  This is used for const blocks in Sequential and var blocks
// in a let scope.
type Def struct {
	Name *ID  `json:"name"`
	Expr Expr `json:"expr"`
	Loc  `json:"loc"`
}

func (*Scope) OpAST()        {}
func (*Parallel) OpAST()     {}
func (*Switch) OpAST()       {}
func (*Sort) OpAST()         {}
func (*Cut) OpAST()          {}
func (*Drop) OpAST()         {}
func (*Head) OpAST()         {}
func (*Tail) OpAST()         {}
func (*Pass) OpAST()         {}
func (*Uniq) OpAST()         {}
func (*Aggregate) OpAST()    {}
func (*Top) OpAST()          {}
func (*Put) OpAST()          {}
func (*OpAssignment) OpAST() {}
func (*OpExpr) OpAST()       {}
func (*Rename) OpAST()       {}
func (*Fuse) OpAST()         {}
func (*Join) OpAST()         {}
func (*Shape) OpAST()        {}
func (*From) OpAST()         {}
func (*Explode) OpAST()      {}
func (*Merge) OpAST()        {}
func (*Over) OpAST()         {}
func (*Search) OpAST()       {}
func (*Where) OpAST()        {}
func (*Yield) OpAST()        {}
func (*Sample) OpAST()       {}
func (*Load) OpAST()         {}
func (*Assert) OpAST()       {}
func (*Output) OpAST()       {}
func (*Debug) OpAST()        {}
func (*Distinct) OpAST()     {}
func (*Delete) OpAST()       {}

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
