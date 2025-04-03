package expr

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"math"
	"regexp"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
)

type Evaluator interface {
	Eval(Context, super.Value) super.Value
}

type Function interface {
	Call(super.Allocator, []super.Value) super.Value
}

type Not struct {
	zctx *super.Context
	expr Evaluator
}

var _ Evaluator = (*Not)(nil)

func NewLogicalNot(zctx *super.Context, e Evaluator) *Not {
	return &Not{zctx, e}
}

func (n *Not) Eval(ectx Context, this super.Value) super.Value {
	val := EvalBool(n.zctx, ectx, this, n.expr)
	if val.IsError() || val.IsNull() {
		return val
	}
	return super.NewBool(!val.Bool())
}

type And struct {
	zctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func NewLogicalAnd(zctx *super.Context, lhs, rhs Evaluator) *And {
	return &And{zctx, lhs, rhs}
}

type Or struct {
	zctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func NewLogicalOr(zctx *super.Context, lhs, rhs Evaluator) *Or {
	return &Or{zctx, lhs, rhs}
}

// EvalBool evaluates e with this and returns the result if it is a bool or error.
// Otherwise, EvalBool returns an error.
func EvalBool(zctx *super.Context, ectx Context, this super.Value, e Evaluator) super.Value {
	val := e.Eval(ectx, this)
	if super.TypeUnder(val.Type()) == super.TypeBool || val.IsError() {
		return val
	}
	return zctx.WrapError("not type bool", val)
}

func (a *And) Eval(ectx Context, this super.Value) super.Value {
	lhs := EvalBool(a.zctx, ectx, this, a.lhs)
	rhs := EvalBool(a.zctx, ectx, this, a.rhs)
	if isfalse(lhs) || isfalse(rhs) {
		// anything AND FALSE = FALSE
		return super.False
	}
	// ERROR AND NULL = ERROR
	// ERROR AND TRUE = ERROR
	if lhs.IsError() {
		return lhs
	}
	if rhs.IsError() {
		return rhs
	}
	if lhs.IsNull() || rhs.IsNull() {
		// NULL AND TRUE = NULL
		return super.NullBool
	}
	return super.True
}

func isfalse(val super.Value) bool {
	return val.Type().ID() == super.IDBool && !val.IsNull() && !val.Bool()
}

func (o *Or) Eval(ectx Context, this super.Value) super.Value {
	lhs := EvalBool(o.zctx, ectx, this, o.lhs)
	rhs := EvalBool(o.zctx, ectx, this, o.rhs)
	if lhs.AsBool() || rhs.AsBool() {
		// anything OR TRUE = TRUE
		return super.True
	}
	if lhs.IsNull() || rhs.IsNull() {
		// NULL OR FALSE = NULL
		// NULL OR ERROR = NULL
		return super.NullBool
	}
	// ERROR OR FALSE = ERROR
	if lhs.IsError() {
		return lhs
	}
	if rhs.IsError() {
		return rhs
	}
	return super.False
}

type In struct {
	zctx      *super.Context
	elem      Evaluator
	container Evaluator
}

func NewIn(zctx *super.Context, elem, container Evaluator) *In {
	return &In{
		zctx:      zctx,
		elem:      elem,
		container: container,
	}
}

func (i *In) Eval(ectx Context, this super.Value) super.Value {
	elem := i.elem.Eval(ectx, this)
	if elem.IsError() {
		return elem
	}
	container := i.container.Eval(ectx, this)
	if container.IsError() {
		return container
	}
	err := container.Walk(func(typ super.Type, body zcode.Bytes) error {
		if coerce.Equal(elem, super.NewValue(typ, body)) {
			return errMatch
		}
		return nil
	})
	switch err {
	case errMatch:
		return super.True
	case nil:
		return super.False
	default:
		return i.zctx.NewError(err)
	}
}

type Equal struct {
	numeric
	equality bool
}

func NewCompareEquality(zctx *super.Context, lhs, rhs Evaluator, operator string) (*Equal, error) {
	e := &Equal{numeric: newNumeric(zctx, lhs, rhs)} //XXX
	switch operator {
	case "==":
		e.equality = true
	case "!=":
	default:
		return nil, fmt.Errorf("unknown equality operator: %s", operator)
	}
	return e, nil
}

func (e *Equal) Eval(ectx Context, this super.Value) super.Value {
	lhsVal, rhsVal, errVal := e.numeric.eval(ectx, this)
	if errVal != nil {
		return *errVal
	}
	if lhsVal.IsNull() || rhsVal.IsNull() {
		return super.NullBool
	}
	result := coerce.Equal(lhsVal, rhsVal)
	if !e.equality {
		result = !result
	}
	if result {
		return super.True
	}
	return super.False
}

type RegexpMatch struct {
	re   *regexp.Regexp
	expr Evaluator
}

func NewRegexpMatch(re *regexp.Regexp, e Evaluator) *RegexpMatch {
	return &RegexpMatch{re, e}
}

func (r *RegexpMatch) Eval(ectx Context, this super.Value) super.Value {
	val := r.expr.Eval(ectx, this)
	switch id := val.Type().ID(); id {
	case super.IDString:
		if val.IsNull() {
			return super.NullBool
		}
		if r.re.Match(val.Bytes()) {
			return super.True
		}
	case super.IDNull:
		return super.NullBool
	}
	return super.False
}

type numeric struct {
	zctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func newNumeric(zctx *super.Context, lhs, rhs Evaluator) numeric {
	return numeric{
		zctx: zctx,
		lhs:  lhs,
		rhs:  rhs,
	}
}

func (n *numeric) evalAndPromote(ectx Context, this super.Value) (super.Value, super.Value, super.Type, *super.Value) {
	lhsVal, rhsVal, errVal := n.eval(ectx, this)
	if errVal != nil {
		return super.Null, super.Null, nil, errVal
	}
	id, err := coerce.Promote(lhsVal, rhsVal)
	if err != nil {
		return super.Null, super.Null, nil, n.zctx.NewError(err).Ptr()
	}
	typ, err := n.zctx.LookupType(id)
	if err != nil {
		return super.Null, super.Null, nil, n.zctx.NewError(err).Ptr()
	}
	if lhsVal.IsNull() || rhsVal.IsNull() {
		return super.Null, super.Null, nil, super.NewValue(typ, nil).Ptr()
	}
	return lhsVal, rhsVal, typ, nil
}

func (n *numeric) eval(ectx Context, this super.Value) (super.Value, super.Value, *super.Value) {
	lhs := n.lhs.Eval(ectx, this)
	if lhs.IsError() {
		return super.Null, super.Null, &lhs
	}
	rhs := n.rhs.Eval(ectx, this)
	if rhs.IsError() {
		return super.Null, super.Null, &rhs
	}
	return enumToIndex(ectx, lhs), enumToIndex(ectx, rhs), nil
}

// enumToIndex converts an enum to its index value.
func enumToIndex(ectx Context, val super.Value) super.Value {
	if _, ok := val.Type().(*super.TypeEnum); ok {
		return super.NewValue(super.TypeUint64, val.Bytes())
	}
	return val
}

type Compare struct {
	zctx *super.Context
	numeric
	convert func(int) bool
}

func NewCompareRelative(zctx *super.Context, lhs, rhs Evaluator, operator string) (*Compare, error) {
	c := &Compare{zctx: zctx, numeric: newNumeric(zctx, lhs, rhs)}
	switch operator {
	case "<":
		c.convert = func(v int) bool { return v < 0 }
	case "<=":
		c.convert = func(v int) bool { return v <= 0 }
	case ">":
		c.convert = func(v int) bool { return v > 0 }
	case ">=":
		c.convert = func(v int) bool { return v >= 0 }
	default:
		return nil, fmt.Errorf("unknown comparison operator: %s", operator)
	}
	return c, nil
}

func (c *Compare) result(result int) super.Value {
	return super.NewBool(c.convert(result))
}

func (c *Compare) Eval(ectx Context, this super.Value) super.Value {
	lhs := c.lhs.Eval(ectx, this)
	if lhs.IsError() {
		return lhs
	}
	rhs := c.rhs.Eval(ectx, this)
	if rhs.IsError() {
		return rhs
	}
	lhs, rhs = lhs.Under(), rhs.Under()
	if lhs.IsNull() || rhs.IsNull() {
		return super.NullBool
	}
	switch lid, rid := lhs.Type().ID(), rhs.Type().ID(); {
	case super.IsNumber(lid) && super.IsNumber(rid):
		return c.result(compareNumbers(lhs, rhs, lid, rid))
	case lid != rid:
		return super.False
	case lid == super.IDBool:
		if lhs.Bool() {
			if rhs.Bool() {
				return c.result(0)
			}

		}
	case lid == super.IDBytes:
		return c.result(bytes.Compare(super.DecodeBytes(lhs.Bytes()), super.DecodeBytes(rhs.Bytes())))
	case lid == super.IDString:
		return c.result(cmp.Compare(super.DecodeString(lhs.Bytes()), super.DecodeString(rhs.Bytes())))
	case lid == super.IDIP:
		return c.result(super.DecodeIP(lhs.Bytes()).Compare(super.DecodeIP(rhs.Bytes())))
	default:
		if bytes.Equal(lhs.Bytes(), rhs.Bytes()) {
			return c.result(0)
		}
	}
	return super.False
}

func compareNumbers(a, b super.Value, aid, bid int) int {
	switch {
	case super.IsFloat(aid):
		return cmp.Compare(a.Float(), toFloat(b))
	case super.IsFloat(bid):
		return cmp.Compare(toFloat(a), b.Float())
	case super.IsSigned(aid):
		av := a.Int()
		if super.IsUnsigned(bid) {
			if av < 0 {
				return -1
			}
			return cmp.Compare(uint64(av), b.Uint())
		}
		return cmp.Compare(av, b.Int())
	case super.IsSigned(bid):
		bv := b.Int()
		if super.IsUnsigned(aid) {
			if bv < 0 {
				return 1
			}
			return cmp.Compare(a.Uint(), uint64(bv))
		}
		return cmp.Compare(a.Int(), bv)
	}
	return cmp.Compare(a.Uint(), b.Uint())
}

type isNullExpr struct {
	eval Evaluator
}

func NewIsNullExpr(e Evaluator) Evaluator {
	return &isNullExpr{e}
}

func (i *isNullExpr) Eval(ectx Context, this super.Value) super.Value {
	val := i.eval.Eval(ectx, this)
	if val.IsError() {
		return val
	}
	return super.NewBool(val.IsNull())

}

func toFloat(val super.Value) float64 { return coerce.ToNumeric[float64](val) }
func toInt(val super.Value) int64     { return coerce.ToNumeric[int64](val) }
func toUint(val super.Value) uint64   { return coerce.ToNumeric[uint64](val) }

type Add struct {
	zctx     *super.Context
	operands numeric
}

type Subtract struct {
	zctx     *super.Context
	operands numeric
}

type Multiply struct {
	zctx     *super.Context
	operands numeric
}

type Divide struct {
	zctx     *super.Context
	operands numeric
}

type Modulo struct {
	zctx     *super.Context
	operands numeric
}

var DivideByZero = errors.New("divide by zero")

// NewArithmetic compiles an expression of the form "expr1 op expr2"
// for the arithmetic operators +, -, *, /
func NewArithmetic(zctx *super.Context, lhs, rhs Evaluator, op string) (Evaluator, error) {
	n := newNumeric(zctx, lhs, rhs)
	switch op {
	case "+":
		return &Add{zctx: zctx, operands: n}, nil
	case "-":
		return &Subtract{zctx: zctx, operands: n}, nil
	case "*":
		return &Multiply{zctx: zctx, operands: n}, nil
	case "/":
		return &Divide{zctx: zctx, operands: n}, nil
	case "%":
		return &Modulo{zctx: zctx, operands: n}, nil
	}
	return nil, fmt.Errorf("unknown arithmetic operator: %s", op)
}

func (a *Add) Eval(ectx Context, this super.Value) super.Value {
	lhsVal, rhsVal, typ, errVal := a.operands.evalAndPromote(ectx, this)
	if errVal != nil {
		return *errVal
	}
	switch id := typ.ID(); {
	case super.IsUnsigned(id):
		return super.NewUint(typ, toUint(lhsVal)+toUint(rhsVal))
	case super.IsSigned(id):
		return super.NewInt(typ, toInt(lhsVal)+toInt(rhsVal))
	case super.IsFloat(id):
		return super.NewFloat(typ, toFloat(lhsVal)+toFloat(rhsVal))
	case id == super.IDString:
		v1, v2 := super.DecodeString(lhsVal.Bytes()), super.DecodeString(rhsVal.Bytes())
		return super.NewValue(typ, super.EncodeString(v1+v2))
	}
	return a.zctx.NewErrorf("type %s incompatible with '+' operator", sup.FormatType(typ))
}

func (s *Subtract) Eval(ectx Context, this super.Value) super.Value {
	lhsVal, rhsVal, typ, errVal := s.operands.evalAndPromote(ectx, this)
	if errVal != nil {
		return *errVal
	}
	switch id := typ.ID(); {
	case super.IsUnsigned(id):
		return super.NewUint(typ, toUint(lhsVal)-toUint(rhsVal))
	case super.IsSigned(id):
		if lhsVal.Type().ID() == super.IDTime && rhsVal.Type().ID() == super.IDDuration {
			// Time minus duration should be time
			typ = super.TypeTime
		} else if id == super.IDTime {
			// Return the difference of two times as a duration.
			typ = super.TypeDuration
		}
		return super.NewInt(typ, toInt(lhsVal)-toInt(rhsVal))
	case super.IsFloat(id):
		return super.NewFloat(typ, toFloat(lhsVal)-toFloat(rhsVal))
	}
	return s.zctx.NewErrorf("type %s incompatible with '-' operator", sup.FormatType(typ))
}

func (m *Multiply) Eval(ectx Context, this super.Value) super.Value {
	lhsVal, rhsVal, typ, errVal := m.operands.evalAndPromote(ectx, this)
	if errVal != nil {
		return *errVal
	}
	switch id := typ.ID(); {
	case super.IsUnsigned(id):
		return super.NewUint(typ, toUint(lhsVal)*toUint(rhsVal))
	case super.IsSigned(id):
		return super.NewInt(typ, toInt(lhsVal)*toInt(rhsVal))
	case super.IsFloat(id):
		return super.NewFloat(typ, toFloat(lhsVal)*toFloat(rhsVal))
	}
	return m.zctx.NewErrorf("type %s incompatible with '*' operator", sup.FormatType(typ))
}

func (d *Divide) Eval(ectx Context, this super.Value) super.Value {
	lhsVal, rhsVal, typ, errVal := d.operands.evalAndPromote(ectx, this)
	if errVal != nil {
		return *errVal
	}
	switch id := typ.ID(); {
	case super.IsUnsigned(id):
		v := toUint(rhsVal)
		if v == 0 {
			return d.zctx.NewError(DivideByZero)
		}
		return super.NewUint(typ, toUint(lhsVal)/v)
	case super.IsSigned(id):
		v := toInt(rhsVal)
		if v == 0 {
			return d.zctx.NewError(DivideByZero)
		}
		return super.NewInt(typ, toInt(lhsVal)/v)
	case super.IsFloat(id):
		v := toFloat(rhsVal)
		if v == 0 {
			return d.zctx.NewError(DivideByZero)
		}
		return super.NewFloat(typ, toFloat(lhsVal)/v)
	}
	return d.zctx.NewErrorf("type %s incompatible with '/' operator", sup.FormatType(typ))
}

func (m *Modulo) Eval(ectx Context, this super.Value) super.Value {
	lhsVal, rhsVal, typ, errVal := m.operands.evalAndPromote(ectx, this)
	if errVal != nil {
		return *errVal
	}
	switch id := typ.ID(); {
	case super.IsUnsigned(id):
		v := toUint(rhsVal)
		if v == 0 {
			return m.zctx.NewError(DivideByZero)
		}
		return super.NewUint(typ, lhsVal.Uint()%v)
	case super.IsSigned(id):
		v := toInt(rhsVal)
		if v == 0 {
			return m.zctx.NewError(DivideByZero)
		}
		return super.NewInt(typ, toInt(lhsVal)%v)
	}
	return m.zctx.NewErrorf("type %s incompatible with '%%' operator", sup.FormatType(typ))
}

type UnaryMinus struct {
	zctx *super.Context
	expr Evaluator
}

func NewUnaryMinus(zctx *super.Context, e Evaluator) *UnaryMinus {
	return &UnaryMinus{
		zctx: zctx,
		expr: e,
	}
}

func (u *UnaryMinus) Eval(ectx Context, this super.Value) super.Value {
	val := u.expr.Eval(ectx, this)
	typ := val.Type()
	if super.IsUnsigned(typ.ID()) {
		switch typ.ID() {
		case super.IDUint8:
			typ = super.TypeInt8
		case super.IDUint16:
			typ = super.TypeInt16
		case super.IDUint32:
			typ = super.TypeInt32
		default:
			typ = super.TypeInt64
		}
		v, ok := coerce.ToInt(val, typ)
		if !ok {
			return u.zctx.WrapError("cannot cast to "+sup.FormatType(typ), val)
		}
		if val.IsNull() {
			return super.NewValue(typ, nil)
		}
		val = super.NewInt(typ, v)
	}
	if val.IsNull() && super.IsNumber(typ.ID()) {
		return val
	}
	switch typ.ID() {
	case super.IDFloat16, super.IDFloat32, super.IDFloat64:
		return super.NewFloat(typ, -val.Float())
	case super.IDInt8:
		v := val.Int()
		if v == math.MinInt8 {
			return u.zctx.WrapError("unary '-' underflow", val)
		}
		return super.NewInt8(int8(-v))
	case super.IDInt16:
		v := val.Int()
		if v == math.MinInt16 {
			return u.zctx.WrapError("unary '-' underflow", val)
		}
		return super.NewInt16(int16(-v))
	case super.IDInt32:
		v := val.Int()
		if v == math.MinInt32 {
			return u.zctx.WrapError("unary '-' underflow", val)
		}
		return super.NewInt32(int32(-v))
	case super.IDInt64:
		v := val.Int()
		if v == math.MinInt64 {
			return u.zctx.WrapError("unary '-' underflow", val)
		}
		return super.NewInt64(-v)
	}
	return u.zctx.WrapError("type incompatible with unary '-' operator", val)
}

func getNthFromContainer(container zcode.Bytes, idx int) (zcode.Bytes, bool) {
	if idx < 0 {
		var length int
		for it := container.Iter(); !it.Done(); it.Next() {
			length++
		}
		idx = length + idx
		if idx < 0 || idx >= length {
			return nil, false
		}
	}
	for i, it := 0, container.Iter(); !it.Done(); i++ {
		zv := it.Next()
		if i == idx {
			return zv, true
		}
	}
	return nil, false
}

func lookupKey(mapBytes, target zcode.Bytes) (zcode.Bytes, bool) {
	for it := mapBytes.Iter(); !it.Done(); {
		key := it.Next()
		val := it.Next()
		if bytes.Equal(key, target) {
			return val, true
		}
	}
	return nil, false
}

// Index represents an index operator "container[index]" where container is
// either an array (with index type integer) or a record (with index type string).
type Index struct {
	zctx      *super.Context
	container Evaluator
	index     Evaluator
}

func NewIndexExpr(zctx *super.Context, container, index Evaluator) Evaluator {
	return &Index{zctx, container, index}
}

func (i *Index) Eval(ectx Context, this super.Value) super.Value {
	container := i.container.Eval(ectx, this)
	index := i.index.Eval(ectx, this)
	switch typ := super.TypeUnder(container.Type()).(type) {
	case *super.TypeArray, *super.TypeSet:
		return indexVector(i.zctx, ectx, super.InnerType(typ), container.Bytes(), index)
	case *super.TypeRecord:
		return indexRecord(i.zctx, ectx, typ, container.Bytes(), index)
	case *super.TypeMap:
		return indexMap(i.zctx, ectx, typ, container.Bytes(), index)
	default:
		return i.zctx.Missing()
	}
}

func indexVector(zctx *super.Context, ectx Context, inner super.Type, vector zcode.Bytes, index super.Value) super.Value {
	id := index.Type().ID()
	if !super.IsInteger(id) {
		return zctx.WrapError("index is not an integer", index)
	}
	if index.IsNull() {
		return zctx.Missing()
	}
	var idx int
	if super.IsSigned(id) {
		idx = int(index.Int())
	} else {
		idx = int(index.Uint())
	}
	bytes, ok := getNthFromContainer(vector, idx)
	if !ok {
		return zctx.Missing()
	}
	return deunion(ectx, inner, bytes)
}

func indexRecord(zctx *super.Context, ectx Context, typ *super.TypeRecord, record zcode.Bytes, index super.Value) super.Value {
	id := index.Type().ID()
	if id != super.IDString {
		return zctx.WrapError("record index is not a string", index)
	}
	field := super.DecodeString(index.Bytes())
	val := super.NewValue(typ, record).Ptr().Deref(field)
	if val == nil {
		return zctx.Missing()
	}
	return *val
}

func indexMap(zctx *super.Context, ectx Context, typ *super.TypeMap, mapBytes zcode.Bytes, key super.Value) super.Value {
	if key.IsMissing() {
		return zctx.Missing()
	}
	if key.Type() != typ.KeyType {
		if union, ok := super.TypeUnder(typ.KeyType).(*super.TypeUnion); ok {
			if tag := union.TagOf(key.Type()); tag >= 0 {
				var b zcode.Builder
				super.BuildUnion(&b, union.TagOf(key.Type()), key.Bytes())
				if valBytes, ok := lookupKey(mapBytes, b.Bytes().Body()); ok {
					return deunion(ectx, typ.ValType, valBytes)
				}
			}
		}
		return zctx.Missing()
	}
	if valBytes, ok := lookupKey(mapBytes, key.Bytes()); ok {
		return deunion(ectx, typ.ValType, valBytes)
	}
	return zctx.Missing()
}

func deunion(ectx Context, typ super.Type, b zcode.Bytes) super.Value {
	if union, ok := typ.(*super.TypeUnion); ok {
		typ, b = union.Untag(b)
	}
	return super.NewValue(typ, b)
}

type Conditional struct {
	zctx      *super.Context
	predicate Evaluator
	thenExpr  Evaluator
	elseExpr  Evaluator
}

func NewConditional(zctx *super.Context, predicate, thenExpr, elseExpr Evaluator) *Conditional {
	return &Conditional{
		zctx:      zctx,
		predicate: predicate,
		thenExpr:  thenExpr,
		elseExpr:  elseExpr,
	}
}

func (c *Conditional) Eval(ectx Context, this super.Value) super.Value {
	val := c.predicate.Eval(ectx, this)
	if val.Type().ID() != super.IDBool {
		return c.zctx.WrapError("?-operator: bool predicate required", val)
	}
	if val.Bool() {
		return c.thenExpr.Eval(ectx, this)
	}
	return c.elseExpr.Eval(ectx, this)
}

type Call struct {
	fn    Function
	exprs []Evaluator
	args  []super.Value
}

func NewCall(fn Function, exprs []Evaluator) *Call {
	return &Call{
		fn:    fn,
		exprs: exprs,
		args:  make([]super.Value, len(exprs)),
	}
}

func (c *Call) Eval(ectx Context, this super.Value) super.Value {
	for k, e := range c.exprs {
		c.args[k] = e.Eval(ectx, this)
	}
	return c.fn.Call(ectx, c.args)
}

type Assignment struct {
	LHS *Lval
	RHS Evaluator
}
