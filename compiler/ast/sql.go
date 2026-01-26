package ast

type SQLQueryBody interface {
	Node
	sqlQueryBodyNode()
}

// SQL Query structure all of which implement SQLQueryBody

type (
	SQLQuery struct {
		Kind    string          `json:"kind" unpack:""`
		With    *SQLWith        `json:"with"`
		Body    SQLQueryBody    `json:"body"`
		OrderBy *SQLOrderBy     `json:"order_by"`
		Limit   *SQLLimitOffset `json:"limit"`
		Loc     `json:"loc"`
	}
	SQLSelect struct {
		Kind      string       `json:"kind" unpack:""`
		Distinct  bool         `json:"distinct"`
		Selection SQLSelection `json:"selection"`
		From      SQLTableExpr `json:"from"`
		Where     Expr         `json:"where"`
		GroupBy   []Expr       `json:"group_by"`
		Having    Expr         `json:"having"`
		Loc       `json:"loc"`
	}
	SQLUnion struct {
		Kind     string       `json:"kind" unpack:""`
		Distinct bool         `json:"distinct"`
		Left     SQLQueryBody `json:"left"`
		Right    SQLQueryBody `json:"right"`
		Loc      `json:"loc"`
	}
	SQLValues struct {
		Kind  string `json:"kind" unpack:""`
		Exprs []Expr `json:"exprs"`
		Loc   `json:"loc"`
	}
)

func (*SQLQuery) sqlQueryBodyNode()  {}
func (*SQLSelect) sqlQueryBodyNode() {}
func (*SQLUnion) sqlQueryBodyNode()  {}
func (*SQLValues) sqlQueryBodyNode() {}

// Structure used by instances of SQLQueryBody

type (
	SQLCTE struct {
		Name         *TableAlias  `json:"name"`
		Materialized bool         `json:"materialized"`
		Body         SQLQueryBody `json:"body"`
		Loc          `json:"loc"`
	}
	SQLLimitOffset struct {
		Limit  Expr `json:"limit"`
		Offset Expr `json:"offset"`
		Loc    `json:"loc"`
	}
	SQLOrderBy struct {
		Exprs []SortExpr `json:"exprs"`
		Loc   `json:"loc"`
	}
	SQLSelection struct {
		Args []SQLAsExpr `json:"args"`
		Loc  `json:"loc"`
	}
	SQLWith struct {
		Recursive bool     `json:"recursive"`
		CTEs      []SQLCTE `json:"ctes"`
		Loc       `json:"loc"`
	}
)

// SQLTableInput is a table expression that can be aliased with an AS clause.
type SQLTableInput interface {
	Node
	sqlTableInputNode()
}

func (*FromItem) sqlTableInputNode() {}
func (*SQLPipe) sqlTableInputNode()  {}

type SQLTableExpr interface {
	Node
	sqlTableExprNode()
}

// SQL table expression structure all of which implement SQLTableExpr

type (
	SQLCrossJoin struct {
		Kind  string       `json:"kind" unpack:""`
		Left  SQLTableExpr `json:"left"`
		Right SQLTableExpr `json:"right"`
		Loc   `json:"loc"`
	}
	SQLFromItem struct {
		Kind       string        `json:"kind" unpack:""`
		Input      SQLTableInput `json:"input"`
		Ordinality *Ordinality   `json:"ordinality"`
		Alias      *TableAlias   `json:"alias"`
		Loc        `json:"loc"`
	}
	// A SQLJoin sources data from the two branches of FromElems where any
	// parent feeds the froms with meta data that can be used in the from-entity
	// expression.  This differs from a pipeline Join where the left input data comes
	// from the parent.
	SQLJoin struct {
		Kind  string       `json:"kind" unpack:""`
		Style string       `json:"style"`
		Left  SQLTableExpr `json:"left"`
		Right SQLTableExpr `json:"right"`
		Cond  JoinCond     `json:"cond"`
		Loc   `json:"loc"`
	}
	// SQLPipe turns a Seq into an SQLQueryBody.  This allows us to put pipe queries inside
	// of SQL.  The parser also uses this structure to embed a single SQLOp inside a SQLPipe
	// as a SQLQueryBody.
	SQLPipe struct {
		Kind string `json:"kind" unpack:""`
		Body Seq    `json:"body"`
		Loc  `json:"loc"`
	}
)

type Ordinality struct {
	Loc `json:"loc"`
}

type TableAlias struct {
	Name    string `json:"name"`
	Columns []*ID  `json:"columns"`
	Loc     `json:"loc"`
}

func (*SQLCrossJoin) sqlTableExprNode() {}
func (*SQLFromItem) sqlTableExprNode()  {}
func (*SQLJoin) sqlTableExprNode()      {}

type JoinCond interface {
	Node
	joinCondNode()
}

type (
	JoinOnCond struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	JoinUsingCond struct {
		Kind   string `json:"kind" unpack:""`
		Fields []*ID  `json:"fields"`
		Loc    `json:"loc"`
	}
)

func (*JoinOnCond) joinCondNode()    {}
func (*JoinUsingCond) joinCondNode() {}

type SQLAsExpr struct {
	Kind  string `json:"kind" unpack:""`
	Label *ID    `json:"label"`
	Expr  Expr   `json:"expr"`
	Loc   `json:"loc"`
}

func (*SQLAsExpr) exprNode() {}
