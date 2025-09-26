package sem

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/sup"
)

type Expr interface {
	ast.Node
	exprNode()
}

type (
	ArrayExpr struct {
		ast.Node
		Elems []ArrayElem
	}
	BadExpr struct {
		ast.Node
	}
	BinaryExpr struct {
		ast.Node
		Op  string // normalized Op string
		LHS Expr
		RHS Expr
	}
	// An ast.Call that has been resolved to an actual function call in expression
	// context... this now refers to the flattened function table (or a built-in) via tag
	CallExpr struct {
		ast.Node
		Tag  string
		Args []Expr
	}
	CondExpr struct {
		ast.Node
		Cond Expr
		Then Expr
		Else Expr
	}
	DotExpr struct {
		ast.Node
		LHS Expr
		RHS string
	}
	IndexExpr struct {
		ast.Node
		Expr  Expr
		Index Expr
	}
	IsNullExpr struct {
		ast.Node
		Expr Expr
	}
	LiteralExpr struct {
		ast.Node
		Value string
	}
	MapCallExpr struct {
		ast.Node
		Expr   Expr
		Lambda *CallExpr
	}
	MapExpr struct {
		ast.Node
		Entries []Entry
	}
	RecordExpr struct {
		ast.Node
		Elems []RecordElem
	}
	RegexpMatchExpr struct {
		ast.Node
		Pattern string
		Expr    Expr
	}
	RegexpSearchExpr struct {
		ast.Node
		Pattern string
		Expr    Expr
	}
	SearchTermExpr struct {
		ast.Node
		Text  string
		Value string
		Expr  Expr
	}
	SetExpr struct {
		ast.Node
		Elems []ArrayElem
	}
	SliceExpr struct {
		ast.Node
		Expr Expr
		From Expr
		To   Expr
	}
	SubqueryExpr struct {
		ast.Node
		Correlated bool
		Array      bool
		Body       Seq
	}
	ThisExpr struct {
		ast.Node
		Path []string
	}
	UnaryExpr struct {
		ast.Node
		Op      string
		Operand Expr
	}
)

// Support structures embedded in Expr nodes

// Entry used by MapExpr
type Entry struct {
	Key   Expr
	Value Expr
}

// The sum type for array, set, and record elements.  There is not an easy way
// of creating a Go pseudo-type

type ArrayElem interface {
	arrayElemNode()
}

type RecordElem interface {
	recordElemNode()
}

type (
	FieldElem struct {
		ast.Node
		Name  string
		Value Expr
	}
	SpreadElem struct {
		ast.Node
		Expr Expr
	}
	ExprElem struct {
		ast.Node
		Expr Expr
	}
)

func (*ExprElem) arrayElemNode()    {}
func (*FieldElem) recordElemNode()  {}
func (*SpreadElem) arrayElemNode()  {}
func (*SpreadElem) recordElemNode() {}

func (*AggFunc) exprNode()          {}
func (*ArrayExpr) exprNode()        {}
func (*BadExpr) exprNode()          {}
func (*BinaryExpr) exprNode()       {}
func (*CondExpr) exprNode()         {}
func (*CallExpr) exprNode()         {}
func (*DotExpr) exprNode()          {}
func (*IndexExpr) exprNode()        {}
func (*IsNullExpr) exprNode()       {}
func (*LiteralExpr) exprNode()      {}
func (*MapCallExpr) exprNode()      {}
func (*MapExpr) exprNode()          {}
func (*RecordExpr) exprNode()       {}
func (*RegexpMatchExpr) exprNode()  {}
func (*RegexpSearchExpr) exprNode() {}
func (*SearchTermExpr) exprNode()   {}
func (*SetExpr) exprNode()          {}
func (*SliceExpr) exprNode()        {}
func (*SubqueryExpr) exprNode()     {}
func (*ThisExpr) exprNode()         {}
func (*UnaryExpr) exprNode()        {}

// FuncRef is a pseudo-expression that represents a function reference as a value.
// It is not used by the runtime (but could be if we wanted to support this).  Instead,
// the semantic pass uses this in a first stage to represent lambda-parameterized functions
// then in a second stage it unrolls them all into regular calls by creating a unique
// new function for each combination of passed in lambdas.
type FuncRef struct {
	ast.Node
	Tag string
}

// CallParam is a pseudo-expression that is like a call but represents the call
// of a FuncRef passed as an argument with the parameter name given by Param.
// It is not used by the runtime (but could be if we wanted to support this).  Instead,
// the semantic pass uses this in a first stage to represent abstract calls to functions
// passed as parameters, then in a second stage it flattens them all into regular calls
// by creating a unique new function for each combination of passed-in lambdas.
type CallParam struct {
	ast.Node
	Param string
	Args  []Expr
}

func (*FuncRef) exprNode()   {}
func (*CallParam) exprNode() {}

func NewThis(n ast.Node, path []string) *ThisExpr {
	return &ThisExpr{Node: n, Path: path}
}

func NewBinaryExpr(n ast.Node, op string, lhs, rhs Expr) *BinaryExpr {
	return &BinaryExpr{
		Node: n,
		Op:   op,
		LHS:  lhs,
		RHS:  rhs,
	}
}

func NewUnaryExpr(n ast.Node, op string, operand Expr) *UnaryExpr {
	return &UnaryExpr{
		Node:    n,
		Op:      op,
		Operand: operand,
	}
}

func NewCall(n ast.Node, tag string, args []Expr) *CallExpr {
	return &CallExpr{
		Node: n,
		Tag:  tag,
		Args: args,
	}
}

func NewStructuredError(n ast.Node, message string, on Expr) Expr {
	rec := &RecordExpr{
		Node: n,
		Elems: []RecordElem{
			&FieldElem{
				Name:  "message",
				Value: &LiteralExpr{Node: n, Value: sup.FormatValue(super.NewString(message))},
			},
			&FieldElem{
				Name:  "on",
				Value: on,
			},
		},
	}
	return &CallExpr{
		Node: n,
		Tag:  "error",
		Args: []Expr{rec},
	}
}
