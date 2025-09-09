package dag

import (
	"encoding/json"

	"github.com/brimdata/super/order"
)

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
		Fn   FnRef  `json:"fn"`
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
	IndexExpr struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Index Expr   `json:"index"`
	}
	IsNullExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Lambda struct {
		Kind    string   `json:"kind" unpack:""`
		Formals []string `json:"formals"`
		Expr    Expr     `json:"expr"`
	}
	Literal struct {
		Kind  string `json:"kind" unpack:""`
		Value string `json:"value"`
	}
	MapCall struct {
		Kind   string `json:"kind" unpack:""`
		Expr   Expr   `json:"expr"`
		Lambda *Call  `json:"lambda"`
	}
	MapExpr struct {
		Kind    string  `json:"kind" unpack:""`
		Entries []Entry `json:"entries"`
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
	Subquery struct {
		Kind       string `json:"kind" unpack:""`
		Correlated bool   `json:"correlated"`
		Body       Seq    `json:"body"`
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

func (c *Call) Name() string {
	if fn, ok := c.Fn.(*FnName); ok {
		return fn.Name
	}
	return ""
}

func (*Agg) exprNode()          {}
func (*ArrayExpr) exprNode()    {}
func (*BadExpr) exprNode()      {}
func (*BinaryExpr) exprNode()   {}
func (*Call) exprNode()         {}
func (*Conditional) exprNode()  {}
func (*Dot) exprNode()          {}
func (*IndexExpr) exprNode()    {}
func (*IsNullExpr) exprNode()   {}
func (*Literal) exprNode()      {}
func (*MapCall) exprNode()      {}
func (*MapExpr) exprNode()      {}
func (*RecordExpr) exprNode()   {}
func (*RegexpMatch) exprNode()  {}
func (*RegexpSearch) exprNode() {}
func (*Search) exprNode()       {}
func (*SetExpr) exprNode()      {}
func (*SliceExpr) exprNode()    {}
func (*Subquery) exprNode()     {}
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

func NewCallByName(name string, args []Expr) *Call {
	return &Call{
		Kind: "Call",
		Fn:   &FnName{Kind: "FnName", Name: name},
		Args: args,
	}
}

func NewThis(path []string) *This {
	return &This{"This", path}
}

func NewUnaryExpr(op string, e Expr) *UnaryExpr {
	return &UnaryExpr{"UnaryExpr", op, e}
}

func NewValues(exprs ...Expr) *Values {
	return &Values{"Values", exprs}
}

func CopyExpr(e Expr) Expr {
	if e == nil {
		panic("CopyExpr nil")
	}
	b, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	var copy Expr
	if err := unpacker.Unmarshal(b, &copy); err != nil {
		panic(err)
	}
	return copy
}
