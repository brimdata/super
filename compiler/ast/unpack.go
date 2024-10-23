package ast

import (
	"encoding/json"
	"fmt"

	astzed "github.com/brimdata/super/compiler/ast/zed"
	"github.com/brimdata/super/pkg/unpack"
)

var unpacker = unpack.New(
	astzed.Array{},
	ArrayExpr{},
	Assert{},
	Assignment{},
	OpAssignment{},
	OpExpr{},
	BinaryExpr{},
	Call{},
	Cast{},
	astzed.CastValue{},
	Conditional{},
	ConstDecl{},
	Cut{},
	Debug{},
	astzed.DefValue{},
	Drop{},
	Explode{},
	astzed.Enum{},
	astzed.Error{},
	Field{},
	HTTPArgs{},
	PoolArgs{},
	FormatArg{},
	From{},
	ExprEntity{},
	FString{},
	FStringExpr{},
	FStringText{},
	FuncDecl{},
	Fuse{},
	Summarize{},
	Grep{},
	Head{},
	ID{},
	astzed.ImpliedValue{},
	IndexExpr{},
	Join{},
	Load{},
	Merge{},
	Output{},
	Over{},
	astzed.Map{},
	MapExpr{},
	Shape{},
	OverExpr{},
	Parallel{},
	Pass{},
	astzed.Primitive{},
	Put{},
	astzed.Record{},
	Agg{},
	Regexp{},
	Glob{},
	RecordExpr{},
	Rename{},
	Scope{},
	Search{},
	astzed.Set{},
	SetExpr{},
	Spread{},
	SliceExpr{},
	Sort{},
	OpDecl{},
	Switch{},
	Tail{},
	Term{},
	Top{},
	astzed.TypeArray{},
	astzed.TypeDef{},
	TypeDecl{},
	astzed.TypeEnum{},
	astzed.TypeError{},
	astzed.TypeMap{},
	astzed.TypeName{},
	astzed.TypeNull{},
	astzed.TypePrimitive{},
	astzed.TypeRecord{},
	astzed.TypeSet{},
	astzed.TypeUnion{},
	astzed.TypeValue{},
	UnaryExpr{},
	Uniq{},
	VectorValue{},
	Where{},
	Yield{},
	Sample{},
	Delete{},
	Lake{},
)

// UnmarshalOp transforms a JSON representation of an operator into an Op.
func UnmarshalOp(buf []byte) (Op, error) {
	var op Op
	if err := unpacker.Unmarshal(buf, &op); err != nil {
		return nil, err
	}
	return op, nil
}

func unmarshalSeq(buf []byte) (Seq, error) {
	var seq Seq
	if err := unpacker.Unmarshal(buf, &seq); err != nil {
		return nil, err
	}
	return seq, nil
}

func UnmarshalObject(anon interface{}) (Seq, error) {
	b, err := json.Marshal(anon)
	if err != nil {
		return nil, fmt.Errorf("internal error: ast.UnmarshalObject: %w", err)
	}
	return unmarshalSeq(b)
}

func Copy(in Op) Op {
	b, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	out, err := UnmarshalOp(b)
	if err != nil {
		panic(err)
	}
	return out
}

func CopySeq(in Seq) Seq {
	b, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	out, err := unmarshalSeq(b)
	if err != nil {
		panic(err)
	}
	return out
}
