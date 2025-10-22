package ast

type Expr interface {
	Node
	exprNode()
}

type (
	ArrayExpr struct {
		Kind  string      `json:"kind" unpack:""`
		Elems []ArrayElem `json:"elems"`
		Loc   `json:"loc"`
	}
	BetweenExpr struct {
		Kind  string `json:"kind" unpack:""`
		Not   bool   `json:"not"`
		Expr  Expr   `json:"expr"`
		Lower Expr   `json:"lower"`
		Upper Expr   `json:"upper"`
		Loc   `json:"loc"`
	}
	// A BinaryExpr is any expression of the form "lhs kind rhs"
	// including arithmetic (+, -, *, /), logical operators (and, or),
	// comparisons (=, !=, <, <=, >, >=), and a dot expression (".") (on records).
	BinaryExpr struct {
		Kind string `json:"kind" unpack:""`
		Op   string `json:"op"`
		LHS  Expr   `json:"lhs"`
		RHS  Expr   `json:"rhs"`
		Loc  `json:"loc"`
	}
	// A CallExpr represents different things dependending on its context.
	// As an operator (when wrapped in an OpExpr), it is either an aggregate
	// with no grouping keys and no duration, or a filter with a function
	// that is boolean valued.  This is determined by the compiler rather than
	// the syntax tree based on the specific functions and aggregators that
	// are defined at compile time.  In expression context, a function call has
	// the standard semantics where it takes one or more arguments and returns a result.
	CallExpr struct {
		Kind  string `json:"kind" unpack:""`
		Func  Expr   `json:"func"`
		Args  []Expr `json:"args"`
		Where Expr   `json:"where"`
		Loc   `json:"loc"`
	}
	CaseExpr struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Whens []When `json:"whens"`
		Else  Expr   `json:"else"`
		Loc   `json:"loc"`
	}
	CastExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Type Type   `json:"type"`
		Loc  `json:"loc"`
	}
	CondExpr struct {
		Kind string `json:"kind" unpack:""`
		Cond Expr   `json:"cond"`
		Then Expr   `json:"then"`
		Else Expr   `json:"else"`
		Loc  `json:"loc"`
	}
	// DoubleQuoteExpr is specialized from the other primitive types because
	// these values can be interpreted either as a string value or an identifier based
	// on SQL vs pipe context.  The semantic pass needs to know the string was
	// originally double quoted to perform this analysis.
	DoubleQuoteExpr struct {
		Kind string `json:"kind" unpack:""`
		Text string `json:"text"`
		Loc  `json:"loc"`
	}
	ExistsExpr struct {
		Kind string `json:"kind" unpack:""`
		Body Seq    `json:"body"`
		Loc  `json:"loc"`
	}
	ExtractExpr struct {
		Kind string `json:"kind" unpack:""`
		Part Expr   `json:"part"`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	FuncNameExpr struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
		Loc  `json:"loc"`
	}
	FStringExpr struct {
		Kind  string        `json:"kind" unpack:""`
		Elems []FStringElem `json:"elems"`
		Loc   `json:"loc"`
	}
	GlobExpr struct {
		Kind    string `json:"kind" unpack:""`
		Pattern string `json:"pattern"`
		Loc     `json:"loc"`
	}
	IDExpr struct {
		Kind string `json:"kind" unpack:""`
		ID   `json:"id"`
	}
	IndexExpr struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Index Expr   `json:"index"`
		Loc   `json:"loc"`
	}
	IsNullExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Not  bool   `json:"not"`
		Loc  `json:"loc"`
	}
	LambdaExpr struct {
		Kind   string `json:"kind" unpack:""`
		Params []*ID  `json:"params"`
		Expr   Expr   `json:"expr"`
		Loc    `json:"loc"`
	}
	MapExpr struct {
		Kind    string     `json:"kind" unpack:""`
		Entries []MapEntry `json:"entries"`
		Loc     `json:"loc"`
	}
	RecordExpr struct {
		Kind  string       `json:"kind" unpack:""`
		Elems []RecordElem `json:"elems"`
		Loc   `json:"loc"`
	}
	RegexpExpr struct {
		Kind    string `json:"kind" unpack:""`
		Pattern string `json:"pattern"`
		Loc     `json:"loc"`
	}
	SearchTermExpr struct {
		Kind  string `json:"kind" unpack:""`
		Text  string `json:"text"`
		Value Any    `json:"value"`
		Loc   `json:"loc"`
	}
	SetExpr struct {
		Kind  string      `json:"kind" unpack:""`
		Elems []ArrayElem `json:"elems"`
		Loc   `json:"loc"`
	}
	SliceExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		From Expr   `json:"from"`
		To   Expr   `json:"to"`
		Loc  `json:"loc"`
	}
	SQLTimeExpr struct {
		Kind  string     `json:"kind" unpack:""`
		Type  string     `json:"type"`
		Value *Primitive `json:"value"`
		Loc   `json:"loc"`
	}
	SubqueryExpr struct {
		Kind  string `json:"kind" unpack:""`
		Body  Seq    `json:"body"`
		Array bool   `json:"array"`
		Loc   `json:"loc"`
	}
	SubstringExpr struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		From Expr   `json:"from"`
		For  Expr   `json:"for"`
		Loc  `json:"loc"`
	}
	TupleExpr struct {
		Kind  string `json:"kind" unpack:""`
		Elems []Expr `json:"elems"`
		Loc   `json:"loc"`
	}
	UnaryExpr struct {
		Kind    string `json:"kind" unpack:""`
		Op      string `json:"op"`
		Operand Expr   `json:"operand"`
		Loc     `json:"loc"`
	}
)

// Support structures embedded in Expr nodes

type ID struct {
	Name string `json:"name"`
	Loc  `json:"loc"`
}

type MapEntry struct {
	Key   Expr `json:"key"`
	Value Expr `json:"value"`
	Loc   `json:"loc"`
}

type When struct {
	Cond Expr `json:"expr"`
	Then Expr `json:"else"`
	Loc  `json:"loc"`
}

type ArrayElem interface {
	arrayElemNode()
}

type RecordElem interface {
	recordElemNode()
}

type (
	FieldElem struct {
		Kind  string `json:"kind" unpack:""`
		Name  *Text  `json:"name"`
		Value Expr   `json:"value"`
		Loc   `json:"loc"`
	}
	SpreadElem struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	ExprElem struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
)

func (*ExprElem) arrayElemNode()    {}
func (*ExprElem) recordElemNode()   {}
func (*FieldElem) recordElemNode()  {}
func (*SpreadElem) arrayElemNode()  {}
func (*SpreadElem) recordElemNode() {}

type FStringElem interface {
	Node
	fStringElemNode()
}

type (
	FStringTextElem struct {
		Kind string `json:"kind" unpack:""`
		Text string `json:"text"`
		Loc  `json:"loc"`
	}
	FStringExprElem struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
)

func (*FStringTextElem) fStringElemNode() {}
func (*FStringExprElem) fStringElemNode() {}

func (*Agg) exprNode()             {}
func (*ArrayExpr) exprNode()       {}
func (*BetweenExpr) exprNode()     {}
func (*BinaryExpr) exprNode()      {}
func (*CallExpr) exprNode()        {}
func (*CaseExpr) exprNode()        {}
func (*CastExpr) exprNode()        {}
func (*CondExpr) exprNode()        {}
func (*DoubleQuoteExpr) exprNode() {}
func (*ExistsExpr) exprNode()      {}
func (*ExtractExpr) exprNode()     {}
func (*FStringExpr) exprNode()     {}
func (*FuncNameExpr) exprNode()    {}
func (*GlobExpr) exprNode()        {}
func (*IDExpr) exprNode()          {}
func (*IndexExpr) exprNode()       {}
func (*IsNullExpr) exprNode()      {}
func (*LambdaExpr) exprNode()      {}
func (*MapExpr) exprNode()         {}
func (*Primitive) exprNode()       {}
func (*RecordExpr) exprNode()      {}
func (*RegexpExpr) exprNode()      {}
func (*SearchTermExpr) exprNode()  {}
func (*SetExpr) exprNode()         {}
func (*SliceExpr) exprNode()       {}
func (*SQLTimeExpr) exprNode()     {}
func (*TupleExpr) exprNode()       {}
func (*TypeValue) exprNode()       {}
func (*UnaryExpr) exprNode()       {}
func (*SubqueryExpr) exprNode()    {}
func (*SubstringExpr) exprNode()   {}
