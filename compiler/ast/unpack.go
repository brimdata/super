package ast

import (
	"encoding/json"
	"fmt"

	"github.com/brimdata/super/pkg/unpack"
)

var unpacker = unpack.New(
	Agg{},
	AggregateOp{},
	ArgExpr{},
	ArgText{},
	Array{},
	ArrayExpr{},
	AssertOp{},
	AssignmentOp{},
	BinaryExpr{},
	CallExpr{},
	CallOp{},
	CaseExpr{},
	CastExpr{},
	CastValue{},
	CondExpr{},
	ConstDecl{},
	CutOp{},
	DateTypeHack{},
	DebugOp{},
	DefaultScan{},
	DefValue{},
	Delete{},
	DoubleQuoteExpr{},
	DropOp{},
	ExplodeOp{},
	ExprElem{},
	ExtractExpr{},
	Enum{},
	Error{},
	ExprEntity{},
	ExprOp{},
	FieldElem{},
	ForkOp{},
	FromOp{},
	FStringExpr{},
	FStringExprElem{},
	FStringTextElem{},
	FuncDecl{},
	FuncNameExpr{},
	FuseOp{},
	GlobExpr{},
	HeadOp{},
	IDExpr{},
	ImpliedValue{},
	IndexExpr{},
	IsNullExpr{},
	JoinOp{},
	LambdaExpr{},
	LoadOp{},
	Map{},
	MapExpr{},
	MergeOp{},
	OpDecl{},
	OutputOp{},
	PassOp{},
	Primitive{},
	PutOp{},
	Record{},
	RecordExpr{},
	RegexpExpr{},
	RenameOp{},
	ScopeOp{},
	SearchOp{},
	SearchTermExpr{},
	Set{},
	SetExpr{},
	ShapesOp{},
	SkipOp{},
	SliceExpr{},
	SortOp{},
	SpreadElem{},
	SubstringExpr{},
	SwitchOp{},
	TailOp{},
	Text{},
	TopOp{},
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
	UniqOp{},
	UnnestOp{},
	ValuesOp{},
	WhereOp{},
	DBMeta{},
	// SuperSQL
	SQLPipe{},
	SQLSelect{},
	SQLCrossJoin{},
	SQLJoin{},
	SQLTimeExpr{},
	SQLUnion{},
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
