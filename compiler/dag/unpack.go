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
	Fork{},
	Func{},
	Fuse{},
	Head{},
	HTTPScan{},
	IndexExpr{},
	IsNullExpr{},
	Join{},
	LakeMetaScan{},
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
	Shape{},
	Skip{},
	SliceExpr{},
	Slicer{},
	Sort{},
	Spread{},
	Switch{},
	Tail{},
	This{},
	Top{},
	UnaryExpr{},
	Uniq{},
	Unnest{},
	UnnestExpr{},
	Values{},
	Var{},
	Vectorize{},
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
