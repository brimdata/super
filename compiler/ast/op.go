package ast

// Op is the interface implemented by all AST operator nodes.
// An Op is a node in the flowgraph that takes values in, operates upon them,
// and produces values as output.
type Op interface {
	Node
	opNode()
}

// A Seq represents a sequence of operators that receive
// a stream of values from their parent into the first operator
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

type (
	AggregateOp struct {
		Kind  string      `json:"kind" unpack:""`
		Limit int         `json:"limit"`
		Keys  Assignments `json:"keys"`
		Aggs  Assignments `json:"aggs"`
		Loc   `json:"loc"`
	}
	AssertOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Text string `json:"text"`
		Loc  `json:"loc"`
	}
	// An AssignmentOp is a list of assignments whose parent operator
	// is unknown: It could be a Aggregate or Put operator. This will be
	// determined in the semantic phase.
	AssignmentOp struct {
		Kind        string      `json:"kind" unpack:""`
		Assignments Assignments `json:"assignments"`
		Loc         `json:"loc"`
	}
	CallOp struct {
		Kind string `json:"kind" unpack:""`
		Name *ID    `json:"name"`
		Args []Expr `json:"args"`
		Loc  `json:"loc"`
	}
	CountOp struct {
		Kind string      `json:"kind" unpack:""`
		Expr *RecordExpr `json:"expr"`
		Loc  `json:"loc"`
	}
	CutOp struct {
		Kind string      `json:"kind" unpack:""`
		Args Assignments `json:"args"`
		Loc  `json:"loc"`
	}
	DebugOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	DistinctOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	DropOp struct {
		Kind string `json:"kind" unpack:""`
		Args []Expr `json:"args"`
		Loc  `json:"loc"`
	}
	ExplodeOp struct {
		Kind string `json:"kind" unpack:""`
		Args []Expr `json:"args"`
		Type Type   `json:"type"`
		As   Expr   `json:"as"`
		Loc  `json:"loc"`
	}
	FromOp struct {
		Kind  string      `json:"kind" unpack:""`
		Elems []*FromElem `json:"elems"`
		Loc   `json:"loc"`
	}
	FuseOp struct {
		Kind string `json:"kind" unpack:""`
		Loc  `json:"loc"`
	}
	// An ExprOp operator is an expression that appears as an operator
	// and requires semantic analysis to determine if it is a filter, a values
	// op, or an aggregation.
	ExprOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	// A ForkOp represents a set of operators that each get
	// a copy of the input from its parent.
	ForkOp struct {
		Kind  string `json:"kind" unpack:""`
		Paths []Seq  `json:"paths"`
		Loc   `json:"loc"`
	}
	HeadOp struct {
		Kind  string `json:"kind" unpack:""`
		Count Expr   `json:"count"`
		Loc   `json:"loc"`
	}
	JoinOp struct {
		Kind       string     `json:"kind" unpack:""`
		Style      string     `json:"style"`
		RightInput Seq        `json:"right_input"`
		Alias      *JoinAlias `json:"alias"`
		Cond       JoinCond   `json:"cond"`
		Loc        `json:"loc"`
	}
	LoadOp struct {
		Kind string  `json:"kind" unpack:""`
		Pool *Text   `json:"pool"`
		Args []OpArg `json:"args"`
		Loc  `json:"loc"`
	}
	MergeOp struct {
		Kind  string     `json:"kind" unpack:""`
		Exprs []SortExpr `json:"exprs"`
		Loc   `json:"loc"`
	}
	OutputOp struct {
		Kind string `json:"kind" unpack:""`
		Name *ID    `json:"name"`
		Loc  `json:"loc"`
	}
	PassOp struct {
		Kind string `json:"kind" unpack:""`
		Loc  `json:"loc"`
	}
	PutOp struct {
		Kind string      `json:"kind" unpack:""`
		Args Assignments `json:"args"`
		Loc  `json:"loc"`
	}
	RenameOp struct {
		Kind string      `json:"kind" unpack:""`
		Args Assignments `json:"args"`
		Loc  `json:"loc"`
	}
	ScopeOp struct {
		Kind  string `json:"kind" unpack:""`
		Decls []Decl `json:"decls"`
		Body  Seq    `json:"body"`
		Loc   `json:"loc"`
	}
	SearchOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	ShapesOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
	SkipOp struct {
		Kind  string `json:"kind" unpack:""`
		Count Expr   `json:"count"`
		Loc   `json:"loc"`
	}
	// SQLOp turns a SQLQueryBody into an Op.  This allows us to put SQL inside of pipes.
	SQLOp struct {
		Kind string       `json:"kind" unpack:""`
		Body SQLQueryBody `json:"body"`
		Loc  `json:"loc"`
	}
	SortOp struct {
		Kind    string     `json:"kind" unpack:""`
		Exprs   []SortExpr `json:"exprs"`
		Reverse bool       `json:"reverse"`
		Loc     `json:"loc"`
	}
	SwitchOp struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Cases []Case `json:"cases"`
		Loc   `json:"loc"`
	}
	TailOp struct {
		Kind  string `json:"kind" unpack:""`
		Count Expr   `json:"count"`
		Loc   `json:"loc"`
	}
	TopOp struct {
		Kind    string     `json:"kind" unpack:""`
		Limit   Expr       `json:"limit"`
		Exprs   []SortExpr `json:"expr"`
		Reverse bool       `json:"reverse"`
		Loc     `json:"loc"`
	}
	UniqOp struct {
		Kind  string `json:"kind" unpack:""`
		Cflag bool   `json:"cflag"`
		Loc   `json:"loc"`
	}
	UnnestOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Body Seq    `json:"body"`
		Loc  `json:"loc"`
	}
	ValuesOp struct {
		Kind  string `json:"kind" unpack:""`
		Exprs []Expr `json:"exprs"`
		Loc   `json:"loc"`
	}
	WhereOp struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
		Loc  `json:"loc"`
	}
)

type (
	DBMeta struct {
		Kind string `json:"kind" unpack:""`
		Meta *Text  `json:"meta"`
		Loc  `json:"loc"`
	}
	DefaultScan struct {
		Kind string `json:"kind" unpack:""`
	}
	Delete struct {
		Kind   string       `json:"kind" unpack:""`
		Pool   string       `json:"pool"`
		Branch string       `json:"branch"`
		Loc    `json:"loc"` // dummy field, not needed except to implement Node
	}
	FileScan struct {
		Kind  string   `json:"kind" unpack:""`
		Paths []string `json:"paths"`
		Loc   `json:"loc"`
	}
)

type Text struct {
	Kind string `json:"kind" unpack:""`
	Text string `json:"value"`
	Loc  `json:"loc"`
}

type FromEntity interface {
	Node
	fromEntityNode()
}

type ExprEntity struct {
	Kind string `json:"kind" unpack:""`
	Expr Expr   `json:"expr"`
	Loc  `json:"loc"`
}

func (*GlobExpr) fromEntityNode()   {}
func (*RegexpExpr) fromEntityNode() {}
func (*ExprEntity) fromEntityNode() {}
func (*DBMeta) fromEntityNode()     {}
func (*Text) fromEntityNode()       {}

type FromElem struct {
	Entity     FromEntity  `json:"entity"`
	Args       []OpArg     `json:"args"`
	Ordinality *Ordinality `json:"ordinality"`
	Alias      *TableAlias `json:"alias"`
	Loc        `json:"loc"`
}

type Ordinality struct {
	Loc `json:"loc"`
}

type TableAlias struct {
	Name    string `json:"name"`
	Columns []*ID  `json:"columns"`
	Loc     `json:"loc"`
}

func (d *DefaultScan) Pos() int { return -1 }
func (d *DefaultScan) End() int { return -1 }

type ArgExpr struct {
	Kind  string `json:"kind" unpack:""`
	Key   string `json:"key"`
	Value Expr   `json:"value"`
	Loc   `json:"loc"`
}

type ArgText struct {
	Kind  string `json:"kind" unpack:""`
	Key   string `json:"key"`
	Value *Text  `json:"value"`
	Loc   `json:"loc"`
}

type OpArg interface {
	Node
	opArgNode()
}

func (*ArgExpr) opArgNode() {}
func (*ArgText) opArgNode() {}

type JoinAlias struct {
	Left, Right *ID
	Loc         `json:"loc"`
}

type SortExpr struct {
	Expr  Expr `json:"expr"`
	Order *ID  `json:"order"`
	Nulls *ID  `json:"nulls"`
	Loc   `json:"loc"`
}

type Case struct {
	Expr Expr `json:"expr"`
	Path Seq  `json:"path"`
}

type Assignment struct {
	LHS Expr `json:"lhs"`
	RHS Expr `json:"rhs"`
	Loc `json:"loc"`
}

type Assignments []Assignment

func (a Assignments) Pos() int { return a[0].Pos() }
func (a Assignments) End() int { return a[len(a)-1].End() }

func (*AggregateOp) opNode()  {}
func (*AssertOp) opNode()     {}
func (*AssignmentOp) opNode() {}
func (*CallOp) opNode()       {}
func (*CountOp) opNode()      {}
func (*CutOp) opNode()        {}
func (*DebugOp) opNode()      {}
func (*Delete) opNode()       {}
func (*DistinctOp) opNode()   {}
func (*DropOp) opNode()       {}
func (*ExplodeOp) opNode()    {}
func (*ExprOp) opNode()       {}
func (*FileScan) opNode()     {}
func (*ForkOp) opNode()       {}
func (*FromOp) opNode()       {}
func (*FuseOp) opNode()       {}
func (*HeadOp) opNode()       {}
func (*JoinOp) opNode()       {}
func (*LoadOp) opNode()       {}
func (*MergeOp) opNode()      {}
func (*OutputOp) opNode()     {}
func (*PassOp) opNode()       {}
func (*PutOp) opNode()        {}
func (*RenameOp) opNode()     {}
func (*ScopeOp) opNode()      {}
func (*SearchOp) opNode()     {}
func (*ShapesOp) opNode()     {}
func (*SkipOp) opNode()       {}
func (*SortOp) opNode()       {}
func (*SQLOp) opNode()        {}
func (*SwitchOp) opNode()     {}
func (*TailOp) opNode()       {}
func (*TopOp) opNode()        {}
func (*UniqOp) opNode()       {}
func (*UnnestOp) opNode()     {}
func (*ValuesOp) opNode()     {}
func (*WhereOp) opNode()      {}

func (*DefaultScan) opNode() {}

// An Agg is an AST node that represents a aggregate function.  The Name
// field indicates the aggregation method while the Expr field indicates
// an expression applied to the incoming records that is operated upon by them
// aggregate function.  If Expr isn't present, then the aggregator doesn't act
// upon a function of the record, e.g., count() counts up records without
// looking into them.
type Agg struct {
	Kind     string `json:"kind" unpack:""`
	Name     string `json:"name"`
	Distinct bool   `json:"distinct"`
	Expr     Expr   `json:"expr"`
	Where    Expr   `json:"where"`
	Loc      `json:"loc"`
}
