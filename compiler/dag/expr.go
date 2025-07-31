package dag

import "github.com/brimdata/super/order"

type (
	Expr interface {
		exprNode()
	}
	RecordElem interface {
		recordElemNode()
	}
	VectorElem interface {
		vectorElemNode()
	}
)

// Exprs

type (
	Agg struct {
		Kind     string `json:"kind" unpack:""`
		Name     string `json:"name"`
		Distinct bool   `json:"distinct"`
		Expr     Expr   `json:"expr"`
		Where    Expr   `json:"where"`
	}
	ArrayExpr struct {
		Kind  string       `json:"kind" unpack:""`
		Elems []VectorElem `json:"elems"`
	}
	// A BadExpr node is a placeholder for an expression containing semantic
	// errors.
	BadExpr struct {
		Kind string `json:"kind" unpack:""`
	}
	BinaryExpr struct {
		Kind string `json:"kind" unpack:""`
		Op   string `json:"op"`
		LHS  Expr   `json:"lhs"`
		RHS  Expr   `json:"rhs"`
	}
	Call struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
		Args []Expr `json:"args"`
	}
	Conditional struct {
		Kind string `json:"kind" unpack:""`
		Cond Expr   `json:"cond"`
		Then Expr   `json:"then"`
		Else Expr   `json:"else"`
	}
	Dot struct {
		Kind string `json:"kind" unpack:""`
		LHS  Expr   `json:"lhs"`
		RHS  string `json:"rhs"`
	}
	Func struct {
		Kind   string   `json:"func" unpack:""`
		Name   string   `json:"name"`
		Params []string `json:"params"`
		Expr   Expr     `json:"expr"`
	}
	IndexExpr struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Index Expr   `json:"index"`
	}
	IsNullExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Literal struct {
		Kind  string `json:"kind" unpack:""`
		Value string `json:"value"`
	}
	MapCall struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Inner Expr   `json:"inner"`
	}
	MapExpr struct {
		Kind    string  `json:"kind" unpack:""`
		Entries []Entry `json:"entries"`
	}
	QueryExpr struct {
		Kind       string `json:"kind" unpack:""`
		Correlated bool   `json:"correlated"`
		Body       Seq    `json:"body"`
	}
	RecordExpr struct {
		Kind  string       `json:"kind" unpack:""`
		Elems []RecordElem `json:"elems"`
	}
	RegexpMatch struct {
		Kind    string `json:"kind" unpack:""`
		Pattern string `json:"pattern"`
		Expr    Expr   `json:"expr"`
	}
	RegexpSearch struct {
		Kind    string `json:"kind" unpack:""`
		Pattern string `json:"pattern"`
		Expr    Expr   `json:"expr"`
	}
	Search struct {
		Kind  string `json:"kind" unpack:""`
		Text  string `json:"text"`
		Value string `json:"value"`
		Expr  Expr   `json:"expr"`
	}
	SetExpr struct {
		Kind  string       `json:"kind" unpack:""`
		Elems []VectorElem `json:"elems"`
	}
	SliceExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		From Expr   `json:"from"`
		To   Expr   `json:"to"`
	}
	SortExpr struct {
		Key   Expr        `json:"key"`
		Order order.Which `json:"order"`
		Nulls order.Nulls `json:"nulls"`
	}
	This struct {
		Kind string   `json:"kind" unpack:""`
		Path []string `json:"path"`
	}
	UnaryExpr struct {
		Kind    string `json:"kind" unpack:""`
		Op      string `json:"op"`
		Operand Expr   `json:"operand"`
	}
)

func (*Agg) exprNode()          {}
func (*ArrayExpr) exprNode()    {}
func (*BadExpr) exprNode()      {}
func (*BinaryExpr) exprNode()   {}
func (*Call) exprNode()         {}
func (*Conditional) exprNode()  {}
func (*Dot) exprNode()          {}
func (*Func) exprNode()         {}
func (*IndexExpr) exprNode()    {}
func (*IsNullExpr) exprNode()   {}
func (*Literal) exprNode()      {}
func (*MapCall) exprNode()      {}
func (*MapExpr) exprNode()      {}
func (*QueryExpr) exprNode()    {}
func (*RecordExpr) exprNode()   {}
func (*RegexpMatch) exprNode()  {}
func (*RegexpSearch) exprNode() {}
func (*Search) exprNode()       {}
func (*SetExpr) exprNode()      {}
func (*SliceExpr) exprNode()    {}
func (*This) exprNode()         {}
func (*UnaryExpr) exprNode()    {}

// Various Expr fields.

type (
	Entry struct {
		Key   Expr `json:"key"`
		Value Expr `json:"value"`
	}
	Field struct {
		Kind  string `json:"kind" unpack:""`
		Name  string `json:"name"`
		Value Expr   `json:"value"`
	}
	Spread struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	VectorValue struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
)

func (*Field) recordElemNode()       {}
func (*Spread) recordElemNode()      {}
func (*Spread) vectorElemNode()      {}
func (*VectorValue) vectorElemNode() {}

func NewBinaryExpr(op string, lhs, rhs Expr) *BinaryExpr {
	return &BinaryExpr{
		Kind: "BinaryExpr",
		Op:   op,
		LHS:  lhs,
		RHS:  rhs,
	}
}

func NewValues(exprs ...Expr) *Values {
	return &Values{"Values", exprs}
}
