package ast

import (
	"encoding/json"
	"fmt"

	"github.com/brimdata/super/pkg/unpack"
)

var unpacker = unpack.New(
	Aggregate{},
	ArgExpr{},
	ArgText{},
	Array{},
	ArrayExpr{},
	Assert{},
	Assignment{},
	OpAssignment{},
	OpExpr{},
	BinaryExpr{},
	Call{},
	CallExtract{},
	CaseExpr{},
	Cast{},
	CastValue{},
	Conditional{},
	ConstDecl{},
	Cut{},
	Debug{},
	DefaultScan{},
	DefValue{},
	DoubleQuote{},
	Drop{},
	Explode{},
	Enum{},
	Error{},
	ExprEntity{},
	FieldExpr{},
	From{},
	FromElem{},
	FString{},
	FStringExpr{},
	FStringText{},
	FuncDecl{},
	Fuse{},
	Head{},
	ID{},
	ImpliedValue{},
	IndexExpr{},
	IsNullExpr{},
	Join{},
	Load{},
	Merge{},
	Skip{},
	Output{},
	Unnest{},
	Map{},
	MapExpr{},
	Shape{},
	Parallel{},
	Pass{},
	Primitive{},
	Put{},
	Record{},
	Agg{},
	Regexp{},
	Glob{},
	RecordExpr{},
	Rename{},
	Scope{},
	Search{},
	Set{},
	SetExpr{},
	Spread{},
	SliceExpr{},
	Sort{},
	Text{},
	OpDecl{},
	Switch{},
	Tail{},
	Term{},
	Top{},
	TypeArray{},
	TypeDef{},
	TypeDecl{},
	TypeEnum{},
	TypeError{},
	TypeMap{},
	TypeName{},
	TypePrimitive{},
	TypeRecord{},
	TypeSet{},
	TypeUnion{},
	TypeValue{},
	UnaryExpr{},
	Uniq{},
	VectorValue{},
	Values{},
	Where{},
	Shapes{},
	Delete{},
	DBMeta{},
	// SuperSQL
	SQLPipe{},
	SQLLimitOffset{},
	SQLSelect{},
	Ordinality{},
	SQLCrossJoin{},
	SQLCast{},
	SQLJoin{},
	SQLTimeValue{},
	SQLUnion{},
	SQLOrderBy{},
	SQLWith{},
	JoinOnCond{},
	JoinUsingCond{},
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

func UnmarshalObject(anon any) (Seq, error) {
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
