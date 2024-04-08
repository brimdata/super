package dag

type (
	Expr interface {
		ExprDAG()
	}
	RecordElem interface {
		recordAST()
	}
	VectorElem interface {
		vectorElem()
	}
)

// Exprs

type (
	Agg struct {
		Kind  string `json:"kind" unpack:""`
		Name  string `json:"name"`
		Expr  Expr   `json:"expr"`
		Where Expr   `json:"where"`
	}
	ArrayExpr struct {
		Kind  string       `json:"kind" unpack:""`
		Elems []VectorElem `json:"elems"`
	}
	Assignment struct {
		Kind string `json:"kind" unpack:""`
		LHS  Expr   `json:"lhs"`
		RHS  Expr   `json:"rhs"`
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
	OverExpr struct {
		Kind  string `json:"kind" unpack:""`
		Defs  []Def  `json:"defs"`
		Exprs []Expr `json:"exprs"`
		Body  Seq    `json:"body"`
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
	This struct {
		Kind string   `json:"kind" unpack:""`
		Path []string `json:"path"`
	}
	UnaryExpr struct {
		Kind    string `json:"kind" unpack:""`
		Op      string `json:"op"`
		Operand Expr   `json:"operand"`
	}
	Var struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
		Slot int    `json:"slot"`
	}
)

func (*Agg) ExprDAG()          {}
func (*ArrayExpr) ExprDAG()    {}
func (*Assignment) ExprDAG()   {}
func (*BinaryExpr) ExprDAG()   {}
func (*Call) ExprDAG()         {}
func (*Conditional) ExprDAG()  {}
func (*Dot) ExprDAG()          {}
func (*Func) ExprDAG()         {}
func (*IndexExpr) ExprDAG()    {}
func (*Literal) ExprDAG()      {}
func (*MapCall) ExprDAG()      {}
func (*MapExpr) ExprDAG()      {}
func (*OverExpr) ExprDAG()     {}
func (*RecordExpr) ExprDAG()   {}
func (*RegexpMatch) ExprDAG()  {}
func (*RegexpSearch) ExprDAG() {}
func (*Search) ExprDAG()       {}
func (*SetExpr) ExprDAG()      {}
func (*SliceExpr) ExprDAG()    {}
func (*This) ExprDAG()         {}
func (*UnaryExpr) ExprDAG()    {}
func (*Var) ExprDAG()          {}

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

func (*Field) recordAST()        {}
func (*Spread) recordAST()       {}
func (*Spread) vectorElem()      {}
func (*VectorValue) vectorElem() {}

func NewBinaryExpr(op string, lhs, rhs Expr) *BinaryExpr {
	return &BinaryExpr{
		Kind: "BinaryExpr",
		Op:   op,
		LHS:  lhs,
		RHS:  rhs,
	}
}

func IsThis(e Expr) bool {
	if p, ok := e.(*This); ok {
		return len(p.Path) == 0
	}
	return false
}

func IsTopLevelField(e Expr) bool {
	_, ok := TopLevelField(e)
	return ok
}

func TopLevelField(e Expr) (string, bool) {
	if b, ok := e.(*This); ok {
		if len(b.Path) == 1 {
			return b.Path[0], true
		}
	}
	return "", false
}
