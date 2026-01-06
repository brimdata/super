package dag

import (
	"fmt"

	"github.com/brimdata/super/pkg/unpack"
)

var unpacker = unpack.New(
	AggExpr{},
	AggregateOp{},
	ArrayExpr{},
	Assignment{},
	BadExpr{},
	BinaryExpr{},
	CallExpr{},
	CombineOp{},
	CommitMetaScan{},
	CondExpr{},
	CountOp{},
	CutOp{},
	DefaultScan{},
	DeleterScan{},
	DeleteScan{},
	DotExpr{},
	DropOp{},
	Field{},
	FileScan{},
	FilterOp{},
	FuncDef{},
	ForkOp{},
	FuseOp{},
	HashJoinOp{},
	HeadOp{},
	HTTPScan{},
	IndexExpr{},
	IsNullExpr{},
	JoinOp{},
	DBMetaScan{},
	ListerScan{},
	LiteralExpr{},
	LoadOp{},
	MapCallExpr{},
	MapExpr{},
	MergeOp{},
	MirrorOp{},
	NullScan{},
	OutputOp{},
	PassOp{},
	PoolMetaScan{},
	PoolScan{},
	PutOp{},
	RecordExpr{},
	RegexpMatchExpr{},
	RegexpSearchExpr{},
	RenameOp{},
	ScatterOp{},
	SearchExpr{},
	SeqScan{},
	SetExpr{},
	SkipOp{},
	SliceExpr{},
	SlicerOp{},
	SortOp{},
	Spread{},
	SubqueryExpr{},
	SwitchOp{},
	TailOp{},
	ThisExpr{},
	TopOp{},
	UnaryExpr{},
	UniqOp{},
	UnnestOp{},
	ValuesOp{},
	VectorValue{},
)

// UnmarshalOp transforms a JSON representation of an operator into an Op.
func UnmarshalOp(buf []byte) (Op, error) {
	var op Op
	if err := unpacker.Unmarshal(buf, &op); err != nil {
		return nil, fmt.Errorf("internal error: JSON object is not a DAG operator: %w", err)
	}
	return op, nil
}
