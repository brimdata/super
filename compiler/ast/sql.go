package ast

type SQLSelect struct {
	Kind      string       `json:"kind" unpack:""`
	Distinct  bool         `json:"distinct"`
	Value     bool         `json:"value"`
	Selection SQLSelection `json:"selection"`
	From      *From        `json:"from"`
	Where     Expr         `json:"where"`
	GroupBy   []Expr       `json:"group_by"`
	Having    Expr         `json:"having"`
	Loc       `json:"loc"`
}

type SQLSelection struct {
	Kind string      `json:"kind" unpack:""`
	Args []SQLAsExpr `json:"args"`
	Loc  `json:"loc"`
}

type SQLValues struct {
	Kind  string `json:"kind" unpack:""`
	Exprs []Expr `json:"exprs"`
	Loc   `json:"loc"`
}

// SQLPipe turns a Seq into an Op.  We need this to put pipes inside
// of SQL expressions.
type SQLPipe struct {
	Kind string `json:"kind" unpack:""`
	Ops  Seq    `json:"ops"`
	Loc  `json:"loc"`
}

type SQLLimitOffset struct {
	Kind   string `json:"kind" unpack:""`
	Op     Op     `json:"op"`
	Limit  Expr   `json:"limit"`
	Offset Expr   `json:"offset"`
	Loc    `json:"loc"`
}

type SQLWith struct {
	Kind      string   `json:"kind" unpack:""`
	Body      Op       `json:"body"`
	Recursive bool     `json:"recursive"`
	CTEs      []SQLCTE `json:"ctes"`
	Loc       `json:"loc"`
}

type SQLCTE struct {
	Name         *ID      `json:"name"`
	Materialized bool     `json:"materialized"`
	Body         *SQLPipe `json:"body"`
	Loc          `json:"loc"`
}

type SQLOrderBy struct {
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
	SQLCrossJoin struct {
		Kind  string    `json:"kind" unpack:""`
		Left  *FromElem `json:"left"`
		Right *FromElem `json:"right"`
		Loc   `json:"loc"`
	}
	SQLUnion struct {
		Kind     string `json:"kind" unpack:""`
		Distinct bool   `json:"distinct"`
		Left     Op     `json:"left"`
		Right    Op     `json:"right"`
		Loc      `json:"loc"`
	}
)

type JoinExpr interface {
	Node
	joinExprNode()
}

type JoinOnExpr struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*JoinOnExpr) joinExprNode() {}

type JoinUsingExpr struct {
	Kind   string `json:"kind" unpack:""`
	Fields []Expr `json:"fields"`
	Loc    `json:"loc"`
}

func (*JoinUsingExpr) joinExprNode() {}

func (*SQLPipe) opNode()        {}
func (*SQLSelect) opNode()      {}
func (*SQLValues) opNode()      {}
func (*SQLCrossJoin) opNode()   {}
func (*SQLJoin) opNode()        {}
func (*SQLUnion) opNode()       {}
func (*SQLOrderBy) opNode()     {}
func (*SQLLimitOffset) opNode() {}
func (*SQLWith) opNode()        {}

type SQLAsExpr struct {
	Kind  string `json:"kind" unpack:""`
	Label *ID    `json:"label"`
	Expr  Expr   `json:"expr"`
	Loc   `json:"loc"`
}

func (*SQLAsExpr) exprNode() {}

type SQLCast struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Type *ID    `json:"type"`
	Loc  `json:"loc"`
}

type SQLSubstring struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	From Expr   `json:"from"`
	For  Expr   `json:"for"`
	Loc  `json:"loc"`
}

func (*SQLCast) exprNode()      {}
func (*SQLSubstring) exprNode() {}
