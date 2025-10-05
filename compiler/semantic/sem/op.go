// Package sem represents a semantic-internal AST representation that
// is used for analysis between the AST and DAG representations.
// This is never serialized so there are no Kind fields or unpacker hooks.
// The sem AST has no scope and all functions, ops, etc. are resolved
// to flattened tables.  Likewise, all SQL  structure is translated to
// native dataflow here with backpointers to the original AST nodes for
// error reporting.
package sem

import (
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/order"
	"github.com/segmentio/ksuid"
)

type Main struct {
	Funcs []*FuncDef
	Body  Seq
}

// Op is the interface implemented by all AST operator nodes.
type Op interface {
	opNode()
	ast.Node
}

// Scanner ops source data and implement Op.
type (
	CommitMetaScan struct {
		ast.Node
		Pool   ksuid.KSUID
		Commit ksuid.KSUID
		Meta   string
		Tap    bool
	}
	DBMetaScan struct {
		ast.Node
		Meta string
	}
	DefaultScan struct {
		ast.Node
	}
	DeleteScan struct {
		ast.Node
		ID     ksuid.KSUID
		Commit ksuid.KSUID
	}
	FileScan struct {
		ast.Node
		Path   string
		Format string
	}
	HTTPScan struct {
		ast.Node
		URL     string
		Format  string
		Method  string
		Headers map[string][]string
		Body    string
	}
	NullScan struct {
		ast.Node
	}
	PoolMetaScan struct {
		ast.Node
		ID   ksuid.KSUID
		Meta string
	}
	PoolScan struct {
		ast.Node
		ID     ksuid.KSUID
		Commit ksuid.KSUID
	}
	RobotScan struct {
		ast.Node
		Expr   Expr
		Format string
	}
)

func (*CommitMetaScan) opNode() {}
func (*DBMetaScan) opNode()     {}
func (*DefaultScan) opNode()    {}
func (*DeleteScan) opNode()     {}
func (*FileScan) opNode()       {}
func (*HTTPScan) opNode()       {}
func (*NullScan) opNode()       {}
func (*PoolMetaScan) opNode()   {}
func (*PoolScan) opNode()       {}
func (*RobotScan) opNode()      {}

type FuncDef struct {
	ast.Node
	Tag    string
	Name   string
	Params []string
	Body   Expr
}

type Seq []Op

func (s *Seq) Prepend(front Op) {
	*s = append([]Op{front}, *s...)
}

func (seq *Seq) Append(op Op) {
	*seq = append(*seq, op)
}

type (
	AggregateOp struct {
		ast.Node
		Limit int
		Keys  []Assignment
		Aggs  []Assignment
	}
	BadOp struct {
		ast.Node
	}
	CutOp struct {
		ast.Node
		Args []Assignment
	}
	DebugOp struct {
		ast.Node
		Expr Expr
	}
	DistinctOp struct {
		ast.Node
		Expr Expr
	}
	DropOp struct {
		ast.Node
		Args []Expr
	}
	ExplodeOp struct {
		ast.Node
		Args []Expr
		Type string
		As   string
	}
	FilterOp struct {
		ast.Node
		Expr Expr
	}
	ForkOp struct {
		ast.Node
		Paths []Seq
	}
	FuseOp struct {
		ast.Node
	}
	HeadOp struct {
		ast.Node
		Count int
	}
	JoinOp struct {
		ast.Node
		Style      string
		LeftAlias  string
		RightAlias string
		Cond       Expr
	}
	LoadOp struct {
		ast.Node
		Pool    ksuid.KSUID
		Branch  string
		Author  string
		Message string
		Meta    string
	}
	MergeOp struct {
		ast.Node
		Exprs []SortExpr
	}
	OutputOp struct {
		ast.Node
		Name string
	}
	PassOp struct {
		ast.Node
	}
	PutOp struct {
		ast.Node
		Args []Assignment
	}
	RenameOp struct {
		ast.Node
		Args []Assignment
	}
	SkipOp struct {
		ast.Node
		Count int
	}
	SortOp struct {
		ast.Node
		Exprs   []SortExpr
		Reverse bool
	}
	SwitchOp struct {
		ast.Node
		Expr  Expr
		Cases []Case
	}
	TailOp struct {
		ast.Node
		Count int
	}
	TopOp struct {
		ast.Node
		Limit   int
		Exprs   []SortExpr
		Reverse bool
	}
	UniqOp struct {
		ast.Node
		Cflag bool
	}
	UnnestOp struct {
		ast.Node
		Expr Expr
		Body Seq
	}
	ValuesOp struct {
		ast.Node
		Exprs []Expr
	}
)

// Suport structs for Ops
type (
	Assignment struct {
		ast.Node
		LHS Expr
		RHS Expr
	}
	// Case paths inside of SwitchOp
	Case struct {
		Expr Expr
		Path Seq
	}
	SortExpr struct {
		ast.Node
		Expr  Expr
		Order order.Which
		Nulls order.Nulls
	}
)

func (*AggregateOp) opNode() {}
func (*BadOp) opNode()       {}
func (*CutOp) opNode()       {}
func (*DebugOp) opNode()     {}
func (*DistinctOp) opNode()  {}
func (*DropOp) opNode()      {}
func (*ExplodeOp) opNode()   {}
func (*FilterOp) opNode()    {}
func (*ForkOp) opNode()      {}
func (*FuseOp) opNode()      {}
func (*HeadOp) opNode()      {}
func (*JoinOp) opNode()      {}
func (*LoadOp) opNode()      {}
func (*MergeOp) opNode()     {}
func (*OutputOp) opNode()    {}
func (*PassOp) opNode()      {}
func (*PutOp) opNode()       {}
func (*RenameOp) opNode()    {}
func (*SkipOp) opNode()      {}
func (*SortOp) opNode()      {}
func (*SwitchOp) opNode()    {}
func (*TailOp) opNode()      {}
func (*TopOp) opNode()       {}
func (*UnnestOp) opNode()    {}
func (*UniqOp) opNode()      {}
func (*ValuesOp) opNode()    {}

type AggFunc struct {
	ast.Node
	Name     string // convert to lower case
	Distinct bool
	Expr     Expr
	Where    Expr
}

func NewValues(n ast.Node, expr ...Expr) *ValuesOp {
	return &ValuesOp{Node: n, Exprs: expr}
}

func NewFilter(n ast.Node, expr Expr) *FilterOp {
	return &FilterOp{Node: n, Expr: expr}
}
