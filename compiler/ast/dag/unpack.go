package dag

import (
	"errors"

	"github.com/brimdata/zed/pkg/unpack"
)

var unpacker = unpack.New(
	ArrayExpr{},
	Assignment{},
	BinaryExpr{},
	Call{},
	Combine{},
	Conditional{},
	Cut{},
	Deleter{},
	Dot{},
	Drop{},
	Explode{},
	Field{},
	FileScan{},
	Filter{},
	Func{},
	Fuse{},
	Summarize{},
	Head{},
	HTTPScan{},
	Join{},
	Lister{},
	Literal{},
	MapExpr{},
	Merge{},
	Shape{},
	SeqScan{},
	Slicer{},
	Spread{},
	Over{},
	OverExpr{},
	Parallel{},
	Pass{},
	PoolScan{},
	Put{},
	Agg{},
	RegexpMatch{},
	RegexpSearch{},
	RecordExpr{},
	Rename{},
	Search{},
	Sequential{},
	SetExpr{},
	Sort{},
	Switch{},
	Tail{},
	This{},
	Top{},
	UnaryExpr{},
	Uniq{},
	Var{},
	VectorValue{},
	Yield{},
)

func UnpackJSON(buf []byte) (interface{}, error) {
	if len(buf) == 0 {
		return nil, nil
	}
	return unpacker.Unmarshal(buf)
}

// UnpackJSONAsOp transforms a JSON representation of an operator into an dag.Op.
func UnpackJSONAsOp(buf []byte) (Op, error) {
	result, err := UnpackJSON(buf)
	if result == nil || err != nil {
		return nil, err
	}
	op, ok := result.(Op)
	if !ok {
		return nil, errors.New("JSON object is not a DAG operator")
	}
	return op, nil
}

func UnpackMapAsOp(m interface{}) (Op, error) {
	object, err := unpacker.UnmarshalObject(m)
	if object == nil || err != nil {
		return nil, err
	}
	op, ok := object.(Op)
	if !ok {
		return nil, errors.New("dag.UnpackMapAsOp: not an Op")
	}
	return op, nil
}
