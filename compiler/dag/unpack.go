package dag

import (
	"fmt"

	"github.com/brimdata/super/pkg/unpack"
)

var unpacker = unpack.New(
	Agg{},
	Aggregate{},
	ArrayExpr{},
	Assignment{},
	BadOp{},
	BadExpr{},
	BinaryExpr{},
	Call{},
	Combine{},
	CommitMetaScan{},
	Conditional{},
	Cut{},
	DefaultScan{},
	Deleter{},
	DeleteScan{},
	Dot{},
	Drop{},
	Explode{},
	Field{},
	FileScan{},
	Filter{},
	FuncDef{},
	FuncName{},
	Fork{},
	Fuse{},
	HashJoin{},
	Head{},
	HTTPScan{},
	IndexExpr{},
	IsNullExpr{},
	Join{},
	DBMetaScan{},
	Lambda{},
	Lister{},
	Literal{},
	Load{},
	MapCall{},
	MapExpr{},
	Merge{},
	Mirror{},
	NullScan{},
	Output{},
	Pass{},
	PoolMetaScan{},
	PoolScan{},
	Put{},
	RecordExpr{},
	RegexpMatch{},
	RegexpSearch{},
	Rename{},
	Scatter{},
	Scope{},
	Search{},
	SeqScan{},
	SetExpr{},
	Skip{},
	SliceExpr{},
	Slicer{},
	Sort{},
	Spread{},
	Subquery{},
	Switch{},
	Tail{},
	This{},
	Top{},
	UnaryExpr{},
	Uniq{},
	Unnest{},
	Values{},
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
