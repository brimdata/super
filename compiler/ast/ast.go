// Package ast declares the types used to represent syntax trees for Zed
// queries.
package ast

import (
	astzed "github.com/brimdata/super/compiler/ast/zed"
)

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
	Kind    string `json:"kind" unpack:""`
	Name    string `json:"name"`
	NamePos int    `json:"name_pos"`
}

func (i *ID) Pos() int { return i.NamePos }
func (i *ID) End() int { return i.NamePos + len(i.Name) }

type Term struct {
	Kind    string     `json:"kind" unpack:""`
	Text    string     `json:"text"`
	TextPos int        `json:"text_pos"`
	Value   astzed.Any `json:"value"`
}

func (t *Term) Pos() int { return t.TextPos }
func (t *Term) End() int { return t.TextPos + len(t.Text) }

type UnaryExpr struct {
	Kind    string `json:"kind" unpack:""`
	Op      string `json:"op"`
	OpPos   int    `json:"op_pos"`
	Operand Expr   `json:"operand"`
}

func (u *UnaryExpr) Pos() int { return u.OpPos }
func (u *UnaryExpr) End() int { return u.Operand.End() }

// A BinaryExpr is any expression of the form "lhs kind rhs"
// including arithmetic (+, -, *, /), logical operators (and, or),
// comparisons (=, !=, <, <=, >, >=), and a dot expression (".") (on records).
type BinaryExpr struct {
	Kind string `json:"kind" unpack:""`
	Op   string `json:"op"`
	LHS  Expr   `json:"lhs"`
	RHS  Expr   `json:"rhs"`
}

func (b *BinaryExpr) Pos() int { return b.LHS.Pos() }
func (b *BinaryExpr) End() int { return b.RHS.End() }

type Conditional struct {
	Kind string `json:"kind" unpack:""`
	Cond Expr   `json:"cond"`
	Then Expr   `json:"then"`
	Else Expr   `json:"else"`
}

func (c *Conditional) Pos() int { return c.Cond.Pos() }
func (c *Conditional) End() int { return c.Else.End() }

// A Call represents different things dependending on its context.
// As a operator, it is either a group-by with no group-by keys and no duration,
// or a filter with a function that is boolean valued.  This is determined
// by the compiler rather than the syntax tree based on the specific functions
// and aggregators that are defined at compile time.  In expression context,
// a function call has the standard semantics where it takes one or more arguments
// and returns a result.
type Call struct {
	Kind   string `json:"kind" unpack:""`
	Name   *ID    `json:"name"`
	Args   []Expr `json:"args"`
	Rparen int    `json:"rparen"`
	Where  Expr   `json:"where"`
}

func (c *Call) Pos() int { return c.Name.Pos() }

func (c *Call) End() int {
	if c.Where != nil {
		return c.Where.End()
	}
	return c.Rparen + 1
}

type Cast struct {
	Kind   string `json:"kind" unpack:""`
	Expr   Expr   `json:"expr"`
	Type   Expr   `json:"type"`
	Rparen int    `json:"rparen"`
}

func (c *Cast) Pos() int { return c.Type.Pos() }
func (c *Cast) End() int { return c.Rparen + 1 }

type IndexExpr struct {
	Kind   string `json:"kind" unpack:""`
	Expr   Expr   `json:"expr"`
	Index  Expr   `json:"index"`
	Rbrack int    `json:"rbrack"`
}

func (i *IndexExpr) Pos() int { return i.Expr.Pos() }
func (i *IndexExpr) End() int { return i.Rbrack + 1 }

type SliceExpr struct {
	Kind   string `json:"kind" unpack:""`
	Expr   Expr   `json:"expr"`
	From   Expr   `json:"from"`
	To     Expr   `json:"to"`
	Rbrack int    `json:"rbrack"`
}

func (s *SliceExpr) Pos() int { return s.Expr.Pos() }
func (s *SliceExpr) End() int { return s.Rbrack + 1 }

type Grep struct {
	Kind       string `json:"kind" unpack:""`
	KeywordPos int    `json:"keyword_pos"`
	Pattern    Expr   `json:"pattern"`
	Expr       Expr   `json:"expr"`
	Rparen     int    `json:"rparen"`
}

func (g *Grep) Pos() int { return g.KeywordPos }
func (g *Grep) End() int { return g.Rparen + 1 }

type Glob struct {
	Kind       string `json:"kind" unpack:""`
	Pattern    string `json:"pattern"`
	PatternPos int    `json:"pattern_pos"`
}

func (g *Glob) Pos() int { return g.PatternPos }
func (g *Glob) End() int { return g.PatternPos + len(g.Pattern) }

type QuotedString struct {
	Kind   string `json:"kind" unpack:""`
	Lquote int    `json:"lquote"`
	Text   string `json:"text"`
}

func (q *QuotedString) Pos() int { return q.Lquote }
func (q *QuotedString) End() int { return q.Lquote + 2 + len(q.Text) }

type Regexp struct {
	Kind       string `json:"kind" unpack:""`
	Pattern    string `json:"pattern"`
	PatternPos int    `json:"pattern_pos"`
}

func (r *Regexp) Pos() int { return r.PatternPos }
func (r *Regexp) End() int { return r.PatternPos + len(r.Pattern) + 2 }

type String struct {
	Kind    string `json:"kind" unpack:""`
	Text    string `json:"text"`
	TextPos int    `json:"text_pos"`
}

func (s *String) Pos() int { return s.TextPos }
func (s *String) End() int { return s.TextPos + len(s.Text) }

type Pattern interface {
	Node
	PatternAST()
}

func (*Glob) PatternAST()         {}
func (*QuotedString) PatternAST() {}
func (*Regexp) PatternAST()       {}
func (*String) PatternAST()       {}

type RecordExpr struct {
	Kind   string       `json:"kind" unpack:""`
	Lbrace int          `json:"lbrace"`
	Elems  []RecordElem `json:"elems"`
	Rbrace int          `json:"rbrace"`
}

func (r *RecordExpr) Pos() int { return r.Lbrace }
func (r *RecordExpr) End() int { return r.Rbrace + 1 }

type RecordElem interface {
	Node
	recordAST()
}

type Field struct {
	Kind    string `json:"kind" unpack:""`
	Name    string `json:"name"`
	NamePos int    `json:"name_pos"`
	Value   Expr   `json:"value"`
}

func (f *Field) Pos() int { return f.NamePos }
func (f *Field) End() int { return f.Value.End() }

type Spread struct {
	Kind     string `json:"kind" unpack:""`
	StartPos int    `json:"start_pos"`
	Expr     Expr   `json:"expr"`
}

func (s *Spread) Pos() int { return s.StartPos }
func (s *Spread) End() int { return s.Expr.End() }

func (*Field) recordAST()  {}
func (*ID) recordAST()     {}
func (*Spread) recordAST() {}

type ArrayExpr struct {
	Kind   string       `json:"kind" unpack:""`
	Lbrack int          `json:"lbrack"`
	Elems  []VectorElem `json:"elems"`
	Rbrack int          `json:"rbrack"`
}

func (a *ArrayExpr) Pos() int { return a.Lbrack }
func (a *ArrayExpr) End() int { return a.Rbrack + 1 }

type SetExpr struct {
	Kind  string       `json:"kind" unpack:""`
	Lpipe int          `json:"lpipe"`
	Elems []VectorElem `json:"elems"`
	Rpipe int          `json:"rpipe"`
}

func (s *SetExpr) Pos() int { return s.Lpipe }
func (s *SetExpr) End() int { return s.Rpipe + 1 }

type VectorElem interface {
	vectorAST()
}

func (*Spread) vectorAST()      {}
func (*VectorValue) vectorAST() {}

type VectorValue struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
}

type MapExpr struct {
	Kind    string      `json:"kind" unpack:""`
	Lpipe   int         `json:"lpipe"`
	Entries []EntryExpr `json:"entries"`
	Rpipe   int         `json:"rpipe"`
}

func (m *MapExpr) Pos() int { return m.Lpipe }
func (m *MapExpr) End() int { return m.Rpipe + 1 }

type EntryExpr struct {
	Key   Expr `json:"key"`
	Value Expr `json:"value"`
}

type OverExpr struct {
	Kind       string `json:"kind" unpack:""`
	KeywordPos int    `json:"keyword"`
	Locals     []Def  `json:"locals"`
	Exprs      []Expr `json:"exprs"`
	Body       Seq    `json:"body"`
}

func (o *OverExpr) Pos() int { return o.KeywordPos }
func (o *OverExpr) End() int { return o.Body.End() }

type FString struct {
	Kind     string        `json:"kind" unpack:""`
	StartPos int           `json:"start_pos"`
	Elems    []FStringElem `json:"elems"`
}

func (f *FString) Pos() int { return f.StartPos }
func (f *FString) End() int {
	if n := len(f.Elems); n > 0 {
		return f.Elems[n-1].End() + 1
	}
	return f.StartPos + 3
}

type FStringElem interface {
	Node
	FStringElem()
}

type FStringText struct {
	Kind    string `json:"kind" unpack:""`
	Text    string `json:"text"`
	TextPos int    `json:"text_pos"`
}

func (f *FStringText) Pos() int { return f.TextPos }
func (f *FStringText) End() int { return f.TextPos + len(f.Text) }

type FStringExpr struct {
	Kind   string `json:"kind" unpack:""`
	Lbrace int    `json:"lbrace"`
	Expr   Expr   `json:"expr"`
	Rbrace int    `json:"rbrace"`
}

func (f *FStringExpr) Pos() int { return f.Lbrace }
func (f *FStringExpr) End() int { return f.Rbrace + 1 }

func (*FStringText) FStringElem() {}
func (*FStringExpr) FStringElem() {}

func (*UnaryExpr) ExprAST()   {}
func (*BinaryExpr) ExprAST()  {}
func (*Conditional) ExprAST() {}
func (*Call) ExprAST()        {}
func (*Cast) ExprAST()        {}
func (*ID) ExprAST()          {}
func (*IndexExpr) ExprAST()   {}
func (*SliceExpr) ExprAST()   {}

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
func (*OverExpr) ExprAST()   {}
func (*FString) ExprAST()    {}

type ConstDecl struct {
	Kind       string `json:"kind" unpack:""`
	KeywordPos int    `json:"keyword_pos"`
	Name       *ID    `json:"name"`
	Expr       Expr   `json:"expr"`
}

func (c *ConstDecl) Pos() int { return c.KeywordPos }
func (c *ConstDecl) End() int { return c.Expr.End() }

type FuncDecl struct {
	Kind       string `json:"kind" unpack:""`
	KeywordPos int    `json:"keyword_pos"`
	Name       *ID    `json:"name"`
	Params     []*ID  `json:"params"`
	Expr       Expr   `json:"expr"`
	Rparen     int    `json:"rparen"`
}

func (f *FuncDecl) Pos() int { return f.KeywordPos }
func (f *FuncDecl) End() int { return f.Rparen }

type OpDecl struct {
	Kind       string `json:"kind" unpack:""`
	KeywordPos int    `json:"keyword_pos"`
	Name       *ID    `json:"name"`
	Params     []*ID  `json:"params"`
	Body       Seq    `json:"body"`
	Rparen     int    `json:"rparen"`
}

func (o *OpDecl) Pos() int { return o.KeywordPos }
func (o *OpDecl) End() int { return o.Rparen }

type TypeDecl struct {
	Kind       string      `json:"kind" unpack:""`
	KeywordPos int         `json:"keyword_pos"`
	Name       *ID         `json:"name"`
	Type       astzed.Type `json:"type"`
}

func (t *TypeDecl) Pos() int { return t.KeywordPos }
func (t *TypeDecl) End() int { return t.Type.End() }

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
	}
	// A Parallel operator represents a set of operators that each get
	// a copy of the input from its parent.
	Parallel struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Paths      []Seq  `json:"paths"`
		Rparen     int    `json:"rparen"`
	}
	Switch struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Expr       Expr   `json:"expr"`
		Cases      []Case `json:"cases"`
		Rparen     int    `json:"rparen"`
	}
	Sort struct {
		Kind       string     `json:"kind" unpack:""`
		KeywordPos int        `json:"keyword_pos"`
		Reverse    bool       `json:"reverse"`
		NullsFirst bool       `json:"nullsfirst"`
		Args       []SortExpr `json:"args"`
	}
	Cut struct {
		Kind       string      `json:"kind" unpack:""`
		KeywordPos int         `json:"keyword_pos"`
		Args       Assignments `json:"args"`
	}
	Drop struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Args       []Expr `json:"args"`
	}
	Explode struct {
		Kind       string      `json:"kind" unpack:""`
		KeywordPos int         `json:"keyword_pos"`
		Args       []Expr      `json:"args"`
		Type       astzed.Type `json:"type"`
		As         Expr        `json:"as"`
	}
	Head struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Count      Expr   `json:"count"`
	}
	Tail struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Count      Expr   `json:"count"`
	}
	Pass struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
	}
	Uniq struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Cflag      bool   `json:"cflag"`
	}
	Summarize struct {
		Kind string `json:"kind" unpack:""`
		// StartPos is not called KeywordPos for Summarize since the "summarize"
		// keyword is optional.
		StartPos int         `json:"start_pos"`
		Limit    int         `json:"limit"`
		Keys     Assignments `json:"keys"`
		Aggs     Assignments `json:"aggs"`
	}
	Top struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Limit      Expr   `json:"limit"`
		Args       []Expr `json:"args"`
		Flush      bool   `json:"flush"`
	}
	Put struct {
		Kind       string      `json:"kind" unpack:""`
		KeywordPos int         `json:"keyword_pos"`
		Args       Assignments `json:"args"`
	}
	Merge struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Expr       Expr   `json:"expr"`
	}
	Over struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Exprs      []Expr `json:"exprs"`
		Locals     []Def  `json:"locals"`
		Body       Seq    `json:"body"`
		Rparen     int    `json:"rparen"`
	}
	Search struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Expr       Expr   `json:"expr"`
	}
	Where struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Expr       Expr   `json:"expr"`
	}
	Yield struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Exprs      []Expr `json:"exprs"`
	}
	// An OpAssignment is a list of assignments whose parent operator
	// is unknown: It could be a Summarize or Put operator. This will be
	// determined in the semantic phase.
	OpAssignment struct {
		Kind        string      `json:"kind" unpack:""`
		Assignments Assignments `json:"assignments"`
	}
	// An OpExpr operator is an expression that appears as an operator
	// and requires semantic analysis to determine if it is a filter, a yield,
	// or an aggregation.
	OpExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Rename struct {
		Kind       string      `json:"kind" unpack:""`
		KeywordPos int         `json:"keyword_pos"`
		Args       Assignments `json:"args"`
	}
	Fuse struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
	}
	Join struct {
		Kind       string      `json:"kind" unpack:""`
		KeywordPos int         `json:"keyword_pos"`
		Style      string      `json:"style"`
		RightInput Seq         `json:"right_input"`
		LeftKey    Expr        `json:"left_key"`
		RightKey   Expr        `json:"right_key"`
		Args       Assignments `json:"args"`
	}
	Sample struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Expr       Expr   `json:"expr"`
	}
	Shape struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
	}
	From struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Trunks     []Seq  `json:"trunks"`
		Rparen     int    `json:"rparen"`
	}
	Load struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Pool       string `json:"pool"`
		Branch     string `json:"branch"`
		Author     string `json:"author"`
		Message    string `json:"message"`
		Meta       string `json:"meta"`
		// XXX This is super hacky but so is this Op. Fix this once we can get
		// positional information for the various options.
		EndPos int `json:"end_pos"` //
	}
	Assert struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Expr       Expr   `json:"expr"`
		Text       string `json:"text"`
	}
	Output struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Name       *ID    `json:"name"`
	}
	Debug struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"keyword_pos"`
		Expr       Expr   `json:"expr"`
	}
)

// Source structure

type (
	File struct {
		Kind       string     `json:"kind" unpack:""`
		KeywordPos int        `json:"keyword_pos"`
		Path       Pattern    `json:"path"`
		Format     string     `json:"format"`
		SortKeys   []SortExpr `json:"sort_keys"`
		EndPos     int        `json:"end_pos"`
	}
	HTTP struct {
		Kind       string      `json:"kind" unpack:""`
		KeywordPos int         `json:"keyword_pos"`
		URL        Pattern     `json:"url"`
		Format     string      `json:"format"`
		SortKeys   []SortExpr  `json:"sort_keys"`
		Method     string      `json:"method"`
		Headers    *RecordExpr `json:"headers"`
		Body       string      `json:"body"`
		EndPos     int         `json:"end_pos"`
	}
	Pool struct {
		Kind       string   `json:"kind" unpack:""`
		KeywordPos int      `json:"keyword_pos"`
		Spec       PoolSpec `json:"spec"`
		EndPos     int      `json:"end_pos"`
	}
	Delete struct {
		Kind string `json:"kind" unpack:""`
	}
)

type PoolSpec struct {
	Pool   Pattern `json:"pool"`
	Commit string  `json:"commit"`
	Meta   string  `json:"meta"`
	Tap    bool    `json:"tap"`
}

type Source interface {
	Node
	Source()
}

func (*Pool) Source()   {}
func (*File) Source()   {}
func (*HTTP) Source()   {}
func (*Pass) Source()   {}
func (*Delete) Source() {}

func (x *Pool) Pos() int { return x.KeywordPos }
func (x *File) Pos() int { return x.KeywordPos }
func (x *HTTP) Pos() int { return x.KeywordPos }
func (*Delete) Pos() int { return 0 }

func (x *Pool) End() int { return x.EndPos }
func (x *File) End() int { return x.EndPos }
func (x *HTTP) End() int { return x.EndPos }
func (*Delete) End() int { return 0 }

type SortExpr struct {
	Kind  string `json:"kind" unpack:""`
	Expr  Expr   `json:"expr"`
	Order *ID    `json:"order"`
	Nulls *ID    `json:"nulls"`
}

func (s SortExpr) Pos() int { return s.Expr.Pos() }
func (s SortExpr) End() int {
	if s.Order != nil {
		s.Order.End()
	}
	return s.Expr.End()
}

type Case struct {
	Expr Expr `json:"expr"`
	Path Seq  `json:"path"`
}

type Assignment struct {
	Kind string `json:"kind" unpack:""`
	LHS  Expr   `json:"lhs"`
	RHS  Expr   `json:"rhs"`
}

func (a *Assignment) Pos() int {
	if a.LHS != nil {
		return a.LHS.Pos()
	}
	return a.RHS.Pos()
}

func (a *Assignment) End() int { return a.RHS.End() }

type Assignments []Assignment

func (a Assignments) Pos() int { return a[0].Pos() }
func (a Assignments) End() int { return a[len(a)-1].End() }

// Def is like Assignment but the LHS is an identifier that may be later
// referenced.  This is used for const blocks in Sequential and var blocks
// in a let scope.
type Def struct {
	Name *ID  `json:"name"`
	Expr Expr `json:"expr"`
}

func (d Def) Pos() int { return d.Name.Pos() }

func (d Def) End() int {
	if d.Expr != nil {
		d.Expr.End()
	}
	return d.Name.End()
}

func (*Scope) OpAST()        {}
func (*Parallel) OpAST()     {}
func (*Switch) OpAST()       {}
func (*Sort) OpAST()         {}
func (*Cut) OpAST()          {}
func (*Drop) OpAST()         {}
func (*Head) OpAST()         {}
func (*Tail) OpAST()         {}
func (*Pool) OpAST()         {}
func (*File) OpAST()         {}
func (*HTTP) OpAST()         {}
func (*Pass) OpAST()         {}
func (*Uniq) OpAST()         {}
func (*Summarize) OpAST()    {}
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
func (*Delete) OpAST()       {}

func (x *Scope) Pos() int {
	if x.Decls != nil {
		return x.Decls[0].End()
	}
	return x.Body.Pos()
}
func (x *Parallel) Pos() int     { return x.KeywordPos }
func (x *Switch) Pos() int       { return x.KeywordPos }
func (x *Sort) Pos() int         { return x.KeywordPos }
func (x *Cut) Pos() int          { return x.KeywordPos }
func (x *Drop) Pos() int         { return x.KeywordPos }
func (x *Head) Pos() int         { return x.KeywordPos }
func (x *Tail) Pos() int         { return x.KeywordPos }
func (x *Pass) Pos() int         { return x.KeywordPos }
func (x *Uniq) Pos() int         { return x.KeywordPos }
func (x *Summarize) Pos() int    { return x.StartPos }
func (x *Top) Pos() int          { return x.KeywordPos }
func (x *Put) Pos() int          { return x.KeywordPos }
func (x *OpAssignment) Pos() int { return x.Assignments[0].Pos() }
func (x *OpExpr) Pos() int       { return x.Expr.Pos() }
func (x *Rename) Pos() int       { return x.KeywordPos }
func (x *Fuse) Pos() int         { return x.KeywordPos }
func (x *Join) Pos() int         { return x.KeywordPos }
func (x *Shape) Pos() int        { return x.KeywordPos }
func (x *From) Pos() int         { return x.KeywordPos }
func (x *Explode) Pos() int      { return x.KeywordPos }
func (x *Merge) Pos() int        { return x.KeywordPos }
func (x *Over) Pos() int         { return x.KeywordPos }
func (x *Search) Pos() int       { return x.KeywordPos }
func (x *Where) Pos() int        { return x.KeywordPos }
func (x *Yield) Pos() int        { return x.KeywordPos }
func (x *Sample) Pos() int       { return x.KeywordPos }
func (x *Load) Pos() int         { return x.KeywordPos }
func (x *Assert) Pos() int       { return x.KeywordPos }
func (x *Output) Pos() int       { return x.KeywordPos }
func (x *Debug) Pos() int        { return x.KeywordPos }

func (x *Scope) End() int    { return x.Body.End() }
func (x *Parallel) End() int { return x.Rparen }
func (x *Switch) End() int   { return x.Rparen }
func (x *Sort) End() int {
	if len(x.Args) == 0 {
		// XXX End is currently broken for Sort since, Exprs can be nil and we
		// don't have positional information on Sort Flags.
		return x.KeywordPos + 5
	}
	return x.Args[len(x.Args)-1].End()
}
func (x *Cut) End() int {
	if len(x.Args) == 0 {
		return x.KeywordPos + 4
	}
	return x.Args[len(x.Args)-1].End()
}
func (x *Drop) End() int {
	if len(x.Args) == 0 {
		return x.KeywordPos + 5
	}
	return x.Args[len(x.Args)-1].End()
}
func (x *Head) End() int {
	if x.Count == nil {
		return x.KeywordPos + 5
	}
	return x.Count.End()
}
func (x *Tail) End() int {
	if x.Count == nil {
		return x.KeywordPos + 5
	}
	return x.Count.End()
}
func (x *Pass) End() int { return x.KeywordPos + 5 }

func (x *Uniq) End() int {
	// XXX End for Uniq is not right since it doesn't not take into account a
	// possible -c flag.
	return x.KeywordPos + 5
}

func (x *Summarize) End() int {
	// XXX End for Summarize isn't right because it current doesn't take into
	// account the existence of the -limit flag. Positions for flags in operators
	// will be addressed in a future PR.
	if len(x.Keys) > 0 {
		return x.Keys[len(x.Keys)-1].End()
	}
	return x.Aggs[len(x.Aggs)-1].End()
}

func (x *Top) End() int {
	// XXX End for Top isn't right because positions do not work for -flush flag.
	// Positions for flags in operators will be addressed in a future PR.
	if len(x.Args) > 0 {
		return x.Args[len(x.Args)-1].End()
	}
	if x.Limit != nil {
		return x.Limit.End()
	}
	return x.KeywordPos + 4
}
func (x *Put) End() int          { return x.Args[len(x.Args)-1].End() }
func (x *OpAssignment) End() int { return x.Assignments[len(x.Assignments)-1].End() }
func (x *OpExpr) End() int       { return x.Expr.End() }
func (x *Rename) End() int       { return -1 }
func (x *Fuse) End() int         { return x.KeywordPos + 5 }
func (x *Join) End() int {
	switch {
	case len(x.Args) > 0:
		return x.Args[len(x.Args)-1].End()
	case x.RightKey != nil:
		return x.RightKey.End()
	}
	return x.LeftKey.End()
}
func (x *Shape) End() int { return x.KeywordPos + 6 }
func (x *From) End() int  { return x.Rparen + 1 }
func (x *Explode) End() int {
	if x.As != nil {
		return x.As.End()
	}
	return x.Type.End()
}
func (x *Merge) End() int { return x.Expr.End() }
func (x *Over) End() int {
	if x.KeywordPos != -1 {
		return x.KeywordPos
	}
	if len(x.Locals) > 0 {
		return x.Locals[len(x.Locals)-1].End()
	}
	return x.Exprs[len(x.Exprs)-1].End()
}
func (x *Search) End() int { return x.Expr.End() }
func (x *Where) End() int  { return x.Expr.End() }
func (x *Yield) End() int  { return x.Exprs[len(x.Exprs)-1].End() }
func (x *Sample) End() int {
	if x.Expr != nil {
		return x.Expr.End()
	}
	return x.KeywordPos + 7
}
func (x *Load) End() int   { return x.EndPos }
func (x *Assert) End() int { return x.Expr.End() }
func (x *Output) End() int { return x.Name.End() }
func (x *Debug) End() int {
	if x.Expr != nil {
		return x.Expr.End()
	}
	return x.KeywordPos + 6
}

// An Agg is an AST node that represents a aggregate function.  The Name
// field indicates the aggregation method while the Expr field indicates
// an expression applied to the incoming records that is operated upon by them
// aggregate function.  If Expr isn't present, then the aggregator doesn't act
// upon a function of the record, e.g., count() counts up records without
// looking into them.
type Agg struct {
	Kind    string `json:"kind" unpack:""`
	Name    string `json:"name"`
	NamePos int    `json:"name_pos"`
	Expr    Expr   `json:"expr"`
	Rparen  int    `json:"rparen"`
	Where   Expr   `json:"where"`
}

func (a *Agg) Pos() int { return a.NamePos }

func (a *Agg) End() int {
	if a.Where != nil {
		return a.Where.End()
	}
	return a.Rparen + 1
}
