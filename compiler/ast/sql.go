package ast

type SQLClause interface {
	sqlClauseNode()
}

type SQLSelect struct {
	Kind      string       `json:"kind" unpack:""`
	Distinct  bool         `json:"distinct"`
	Value     bool         `json:"value"`
	Selection SQLSelection `json:"selection"`
	From      *FromOp      `json:"from"` // XXX from clause?
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

// SQLPipe turns a Seq into an SQLClause.  This allows us to put pipes inside
// of SQL.
type SQLPipe struct {
	Kind string `json:"kind" unpack:""`
	Body Seq    `json:"body"`
	Loc  `json:"loc"`
}

// SQLOp turns a SQLClause into an Op.  This allows us to put SQL inside of pipes.
type SQLOp struct {
	Kind string    `json:"kind" unpack:""`
	Body SQLClause `json:"body"`
	Loc  `json:"loc"`
}

func (*SQLOp) opNode() {}

type SQLLimitOffset struct {
	Kind   string    `json:"kind" unpack:""`
	Body   SQLClause `json:"body"`
	Limit  Expr      `json:"limit"`
	Offset Expr      `json:"offset"`
	Loc    `json:"loc"`
}

type SQLWith struct {
	Body      SQLClause `json:"body"`
	Recursive bool      `json:"recursive"`
	CTEs      []SQLCTE  `json:"ctes"`
	Loc       `json:"loc"`
}

type SQLCTE struct {
	Name         *ID       `json:"name"`
	Materialized bool      `json:"materialized"`
	Body         SQLClause `json:"body"`
	Loc          `json:"loc"`
}

type SQLOrderBy struct {
	Kind  string     `json:"kind" unpack:""`
	Body  SQLClause  `json:"body"`
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
		Cond  JoinCond  `json:"cond"`
		Loc   `json:"loc"`
	}
	SQLCrossJoin struct {
		Kind  string    `json:"kind" unpack:""`
		Left  *FromElem `json:"left"`
		Right *FromElem `json:"right"`
		Loc   `json:"loc"`
	}
	SQLUnion struct {
		Kind     string    `json:"kind" unpack:""`
		Distinct bool      `json:"distinct"`
		Left     SQLClause `json:"left"`
		Right    SQLClause `json:"right"`
		Loc      `json:"loc"`
	}
)

type JoinCond interface {
	Node
	joinCondNode()
}

type JoinOnCond struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*JoinOnCond) joinCondNode() {}

type JoinUsingCond struct {
	Kind   string `json:"kind" unpack:""`
	Fields []Expr `json:"fields"`
	Loc    `json:"loc"`
}

func (*JoinUsingCond) joinCondNode() {}

func (*SQLPipe) sqlClauseNode()        {}
func (*SQLSelect) sqlClauseNode()      {}
func (*SQLValues) sqlClauseNode()      {}
func (*SQLCrossJoin) sqlClauseNode()   {}
func (*SQLJoin) sqlClauseNode()        {}
func (*SQLUnion) sqlClauseNode()       {}
func (*SQLOrderBy) sqlClauseNode()     {}
func (*SQLLimitOffset) sqlClauseNode() {}

func (*FromOp) sqlClauseNode() {} //XXX

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
	Type Type   `json:"type"`
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
