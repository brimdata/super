package ast

type Select struct {
	Kind      string    `json:"kind" unpack:""`
	Distinct  bool      `json:"distinct"`
	Value     bool      `json:"value"`
	Selection Selection `json:"selection"`
	From      *From     `json:"from"`
	Where     Expr      `json:"where"`
	GroupBy   []Expr    `json:"group_by"`
	Having    Expr      `json:"having"`
	Loc       `json:"loc"`
}

type Selection struct {
	Kind string   `json:"kind" unpack:""`
	Args []AsExpr `json:"args"`
	Loc  `json:"loc"`
}

// SQLPipe turns a Seq into an Op.  We need this to put pipes inside
// of SQL expressions.
type SQLPipe struct {
	Kind string `json:"kind" unpack:""`
	Ops  Seq    `json:"ops"`
	Loc  `json:"loc"`
}

type Limit struct {
	Kind   string `json:"kind" unpack:""`
	Op     Op     `json:"op"`
	Count  Expr   `json:"count"`
	Offset Expr   `json:"offset"`
	Loc    `json:"loc"`
}

type With struct {
	Kind      string `json:"kind" unpack:""`
	Body      Op     `json:"body"`
	Recursive bool   `json:"recursive"`
	CTEs      []CTE  `json:"ctes"`
	Loc       `json:"loc"`
}

type CTE struct {
	Name         string `json:"name"`
	Materialized *bool  `json:"materialized"`
	Op           Op     `json:"op"`
	Loc          `json:"loc"`
}

type OrderBy struct {
	Kind  string     `json:"kind" unpack:""`
	Op    Op         `json:"op"`
	Exprs []SortExpr `json:"exprs"`
	Loc   `json:"loc"`
}

type (
	// A SQLJoin sources data from the two branches of FromElems where any
	// parent feeds the froms with meta data that can be used in the from-entity
	// expression.  This differs from a pipeline Join where the left input data comes
	// from the parent.
	SQLJoin struct {
		Kind  string    `json:"kind" unpack:""`
		Style string    `json:"style"`
		Left  *FromElem `json:"left"`
		Right *FromElem `json:"right"`
		Cond  JoinExpr  `json:"cond"`
		Loc   `json:"loc"`
	}
	CrossJoin struct {
		Kind  string    `json:"kind" unpack:""`
		Left  *FromElem `json:"left"`
		Right *FromElem `json:"right"`
		Loc   `json:"loc"`
	}
	Union struct {
		Kind     string `json:"kind" unpack:""`
		Distinct bool   `json:"distinct"`
		Left     Op     `json:"left"`
		Right    Op     `json:"right"`
		Loc      `json:"loc"`
	}
)

type JoinExpr interface {
	Node
	joinExpr()
}

type JoinOnExpr struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*JoinOnExpr) joinExpr() {}

type JoinUsingExpr struct {
	Kind   string `json:"kind" unpack:""`
	Fields []Expr `json:"fields"`
	Loc    `json:"loc"`
}

func (*JoinUsingExpr) joinExpr() {}

func (*SQLPipe) OpAST()   {}
func (*Select) OpAST()    {}
func (*CrossJoin) OpAST() {}
func (*SQLJoin) OpAST()   {}
func (*Union) OpAST()     {}
func (*OrderBy) OpAST()   {}
func (*Limit) OpAST()     {}
func (*With) OpAST()      {}

type AsExpr struct {
	Kind string `json:"kind" unpack:""`
	ID   *ID    `json:"id"`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*AsExpr) ExprAST() {}

type SQLCast struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Type *ID    `json:"type"`
	Loc  `json:"loc"`
}

func (*SQLCast) ExprAST() {}
