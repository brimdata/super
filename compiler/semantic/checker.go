package semantic

import (
	"errors"
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/sup"
)

//XXX a lot of type problems should be warnings... e.g., a predicate that doesn't
// encounter any relevant data because it's part of a larger query that might encounter
// such data some of the time

type checker struct {
	sctx    *super.Context //XXX?
	funcs   map[string]*sem.FuncDef
	bad     bool
	unknown *super.TypeError
	estack  []errlist
}

func newChecker(sctx *super.Context, funcs map[string]*sem.FuncDef) *checker {
	return &checker{
		sctx:    sctx,
		funcs:   funcs,
		unknown: sctx.LookupTypeError(sctx.MustLookupTypeRecord(nil)),
	}
}

func (c *checker) check(r reporter, seq sem.Seq) {
	c.epush()
	c.seq(c.unknown, seq)
	errs := c.epop()
	errs.flushErrs(r)
}

func (c *checker) seq(typ super.Type, seq sem.Seq) super.Type {
	for len(seq) > 0 {
		if fork, ok := seq[0].(*sem.ForkOp); ok && len(seq) >= 2 {
			if join, ok := seq[1].(*sem.JoinOp); ok {
				typ = c.join(c.fork(typ, fork), join)
				seq = seq[2:]
				continue
			}
		}
		if swtch, ok := seq[0].(*sem.SwitchOp); ok && len(seq) >= 2 {
			if join, ok := seq[1].(*sem.JoinOp); ok {
				typ = c.join(c.swtch(typ, swtch), join)
				seq = seq[2:]
				continue
			}
		}
		typ = c.op(typ, seq[0])
		seq = seq[1:]
	}
	return typ
}

func (c *checker) op(typ super.Type, op sem.Op) super.Type {
	switch op := op.(type) {
	//
	// Scanners first
	//
	case *sem.DefaultScan:
		return c.unknown //XXX should get type from readers interface
	case *sem.FileScan:
		// XXX should have been set by translator so that SQL schemas could b e
		// managed
		if op.Type == nil {
			return c.unknown
		}
		return typ
	case *sem.HTTPScan,
		*sem.PoolScan,
		*sem.RobotScan,
		*sem.DBMetaScan,
		*sem.PoolMetaScan,
		*sem.CommitMetaScan,
		*sem.DeleteScan:
		return c.unknown
	case *sem.NullScan:
		return super.TypeNull
	//
	// Ops in alphabetical oder
	//
	case *sem.AggregateOp:
		aggPaths := c.assignments(typ, op.Aggs)
		keyPaths := c.assignments(typ, op.Keys)
		return c.pathsToType(append(keyPaths, aggPaths...))
	case *sem.BadOp:
		c.bad = true
		return c.unknown
	case *sem.CutOp:
		return c.pathsToType(c.assignments(typ, op.Args))
	case *sem.DebugOp:
		c.expr(typ, op.Expr)
		return typ
	case *sem.DistinctOp:
		c.expr(typ, op.Expr)
		return typ
	case *sem.DropOp:
		drops := c.lvalsToPaths(op.Args)
		if drops == nil {
			return c.unknown
		}
		return c.dropPaths(typ, drops)
	case *sem.ExplodeOp:
		// TBD
		return c.unknown
	case *sem.FilterOp:
		c.boolean(op.Expr, c.expr(typ, op.Expr))
		return typ
	case *sem.ForkOp:
		return c.fuse(c.fork(typ, op))
	case *sem.FuseOp:
		return typ
	case *sem.HeadOp:
		return typ
	case *sem.LoadOp:
		return c.unknown
	case *sem.MergeOp:
		c.sortExprs(typ, op.Exprs)
		return typ
	case *sem.JoinOp:
		c.error(op, errors.New("join requires two query inputs"))
		return c.unknown
	case *sem.OutputOp:
		return typ
	case *sem.PassOp:
		return typ
	case *sem.PutOp:
		fields := c.assignments(typ, op.Args)
		return c.putPaths(typ, fields)
	case *sem.RenameOp:
		// TBD
		return c.unknown
	case *sem.SkipOp:
		return typ
	case *sem.SortOp:
		return typ
	case *sem.SwitchOp:
		var types []super.Type
		exprType := c.expr(typ, op.Expr)
		for _, cs := range op.Cases {
			c.expr(exprType, cs.Expr)
			types = append(types, c.seq(typ, cs.Path))
		}
		return c.fuse(types)
	case *sem.TailOp:
		return typ
	case *sem.TopOp:
		c.sortExprs(typ, op.Exprs)
		return typ
	case *sem.UniqOp:
		return typ
	case *sem.UnnestOp:
		return c.seq(c.unnest(op.Expr, c.expr(typ, op.Expr)), op.Body)
	case *sem.ValuesOp:
		return c.fuse(c.exprs(typ, op.Exprs))
	default:
		panic(op)
	}
}

func (c *checker) fork(typ super.Type, fork *sem.ForkOp) []super.Type {
	var types []super.Type
	for _, seq := range fork.Paths {
		types = append(types, c.seq(typ, seq))
	}
	return types
}

func (c *checker) swtch(typ super.Type, op *sem.SwitchOp) []super.Type {
	var types []super.Type
	exprType := c.expr(typ, op.Expr)
	for _, cs := range op.Cases {
		c.expr(exprType, cs.Expr)
		types = append(types, c.seq(typ, cs.Path))
	}
	return types
}

func (c *checker) join(types []super.Type, op *sem.JoinOp) super.Type {
	if len(types) != 2 {
		c.error(op, errors.New("join requires two query inputs"))
	}
	typ := c.sctx.MustLookupTypeRecord([]super.Field{
		{Name: op.LeftAlias, Type: types[0]},
		{Name: op.RightAlias, Type: types[1]},
	})
	c.expr(typ, op.Cond)
	return typ
}

func (c *checker) unnest(loc ast.Node, typ super.Type) super.Type {
	c.epush()
	typ, ok := c.unnestCheck(loc, typ)
	errs := c.epop()
	if !ok {
		c.ekeep(errs)
	}
	return typ
}

func (c *checker) unnestCheck(loc ast.Node, typ super.Type) (super.Type, bool) {
	switch typ := super.TypeUnder(typ).(type) {
	case *super.TypeError:
		if isUnknown(typ) {
			return c.unknown, true
		}
		c.error(loc, errors.New("unnested record cannot be an error"))
		return c.unknown, false
	case *super.TypeUnion:
		var types []super.Type
		var ok bool
		for _, t := range typ.Types {
			typ, tok := c.unnestCheck(loc, t)
			if tok {
				types = append(types, typ)
				ok = true
			}
		}
		return c.fuse(types), ok
	case *super.TypeArray:
		return typ.Type, true
	case *super.TypeRecord:
		if len(typ.Fields) != 2 {
			c.error(loc, errors.New("unnested record must have two fields"))
			return c.unknown, false
		}
		arrayField := typ.Fields[1]
		if isUnknown(arrayField.Type) {
			return typ, true
		}
		arrayType, ok := super.TypeUnder(arrayField.Type).(*super.TypeArray)
		if !ok {
			c.error(loc, errors.New("unnested record must have array for second field"))
			return c.unknown, false
		}
		fields := []super.Field{typ.Fields[0], {Name: arrayField.Name, Type: arrayType.Type}}
		return c.sctx.MustLookupTypeRecord(fields), true
	default:
		c.error(loc, errors.New("unnest value must be array or record"))
		return c.unknown, false
	}
}

// XXX assignments returns a set of fields and paths where we can analyze the LHS
// and determined a dotted path. If LHS is more complex than a dotted path (e.g.,
// depends on the input data, e.g., "put this[fld]:=10"), then that path slot is null
func (c *checker) assignments(in super.Type, assignments []sem.Assignment) []pathType {
	var paths []pathType
	for _, a := range assignments {
		var path []string
		if this, ok := a.LHS.(*sem.ThisExpr); ok {
			path = this.Path
		}
		typ := c.expr(in, a.RHS)
		paths = append(paths, pathType{path, typ})
	}
	return paths
}

func (c *checker) sortExprs(typ super.Type, exprs []sem.SortExpr) {
	for _, se := range exprs {
		c.expr(typ, se.Expr)
	}
}

func (c *checker) exprs(typ super.Type, exprs []sem.Expr) []super.Type {
	var types []super.Type
	for _, e := range exprs {
		types = append(types, c.expr(typ, e))
	}
	return types
}

func (c *checker) expr(typ super.Type, e sem.Expr) super.Type {
	switch e := e.(type) {
	case nil:
		return c.unknown
	case *sem.AggFunc:
		c.expr(typ, e.Expr)
		c.expr(typ, e.Where)
		//XXX lookup AggFunc and return proper type
		return c.unknown
	case *sem.ArrayExpr:
		return c.sctx.LookupTypeArray(c.arrayElems(typ, e.Elems))
	case *sem.BadExpr:
		c.bad = true
		return c.unknown
	case *sem.BinaryExpr:
		lhs := c.expr(typ, e.LHS)
		rhs := c.expr(typ, e.RHS)
		switch e.Op {
		case "and", "or":
			c.logical(e.LHS, e.RHS, lhs, rhs)
			return super.TypeBool
		case "in":
			c.in(e.LHS, e.RHS, lhs, rhs)
			return super.TypeBool
		case "==", "!=":
			return c.equality(lhs, rhs)
		case "<", "<=", ">", ">=":
			return c.comparison(lhs, rhs)
		case "+", "-", "*", "/", "%":
			if e.Op == "+" {
				return c.plus(e.LHS, e.RHS, lhs, rhs)
			}
			return c.arithmetic(e.LHS, e.RHS, lhs, rhs)
		default:
			panic(e.Op)
		}
	case *sem.CallExpr:
		for _, e := range e.Args {
			//XXX collect up types and apply to function type checkers
			c.expr(typ, e)
		}
		// TBD
		return c.unknown
	case *sem.CondExpr:
		c.boolean(e.Cond, c.expr(typ, e.Cond))
		return c.fuse([]super.Type{c.expr(typ, e.Then), c.expr(typ, e.Else)})
	case *sem.DotExpr:
		typ, _ := c.deref(e.Node, c.expr(typ, e.LHS), e.RHS)
		return typ
	case *sem.FuncRef:
		// TBD
		return c.unknown
	case *sem.IndexExpr:
		return c.indexOf(e.Expr, e.Index, c.expr(typ, e.Expr), c.expr(typ, e.Index))
	case *sem.IsNullExpr:
		c.expr(typ, e.Expr)
		return super.TypeBool
	case *sem.LiteralExpr:
		if val, err := sup.ParseValue(c.sctx, e.Value); err == nil {
			return val.Type()
		}
		return c.unknown
	case *sem.MapCallExpr:
		// check that container is an array/set and that container element type
		// is compatible with function argument... do this after we have function
		// type checking... we need to do functions in two passes because the lambdas
		// need to be macro-expanded (maybe do this with rentrancy?... if we call funcDecl
		// resolve, seems like it should work)
		c.expr(typ, e.Expr)
		c.expr(typ, e.Lambda)
		return c.unknown //XXX
	case *sem.MapExpr:
		// fuser could take type at a time instead of array
		var keyTypes []super.Type
		var valTypes []super.Type
		for _, entry := range e.Entries {
			keyTypes = append(keyTypes, c.expr(typ, entry.Key))
			valTypes = append(valTypes, c.expr(typ, entry.Value))
		}
		return c.sctx.LookupTypeMap(c.fuse(keyTypes), c.fuse(valTypes))
	case *sem.RecordExpr:
		return c.recordElems(typ, e.Elems)
	case *sem.RegexpMatchExpr:
		//XXX what's different between regexpmatch and regexp search?
		//XXX check that typ has something stringy in it i.e., strings or records (field names)
		c.expr(typ, e.Expr)
		return super.TypeBool
	case *sem.RegexpSearchExpr:
		//XXX check that typ has something stringy in it i.e., strings or records (field names)
		c.expr(typ, e.Expr) //XXX
		return super.TypeBool
	case *sem.SearchTermExpr:
		//XXX check that typ has something stringy in it i.e., strings or records (field names)
		c.expr(typ, e.Expr)
		return super.TypeBool
	case *sem.SetExpr:
		return c.sctx.LookupTypeArray(c.arrayElems(typ, e.Elems))
	case *sem.SliceExpr:
		c.integer(e.From, c.expr(typ, e.From))
		c.integer(e.To, c.expr(typ, e.To))
		container := c.expr(typ, e.Expr)
		c.sliceable(e.Expr, container)
		return container
	case *sem.SubqueryExpr:
		//XXX fix this
		// correlated vs non-correlated
		typ = c.seq(typ, e.Body)
		if e.Array {
			typ = c.sctx.LookupTypeArray(typ)
		}
		return typ
	case *sem.ThisExpr:
		for _, field := range e.Path {
			if e.Node == nil {
				panic(e)
			}
			typ, _ = c.deref(e.Node, typ, field)
		}
		return typ
	case *sem.UnaryExpr:
		typ = c.expr(typ, e.Operand)
		switch e.Op {
		case "-":
			c.number(e.Operand, typ)
			return typ
		case "!":
			c.boolean(e, typ)
			return super.TypeBool
		default:
			panic(e.Op)
		}
	default:
		panic(e)
	}
}

func (c *checker) arrayElems(typ super.Type, elems []sem.ArrayElem) super.Type {
	fuser := c.newFuser()
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			fuser.fuse(c.expr(typ, elem.Expr))
		case *sem.ExprElem:
			fuser.fuse(c.expr(typ, elem.Expr))
		default:
			panic(elem)
		}
	}
	return fuser.Type(c)
}

func (c *checker) recordElems(typ super.Type, elems []sem.RecordElem) super.Type {
	fuser := c.newFuser()
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			fuser.fuse(c.expr(typ, elem.Expr))
		case *sem.FieldElem:
			column := super.Field{Name: elem.Name, Type: c.expr(typ, elem.Value)}
			fuser.fuse(c.sctx.MustLookupTypeRecord([]super.Field{column}))
		default:
			panic(elem)
		}
	}
	return fuser.Type(c)
}

type pathType struct {
	elems []string
	typ   super.Type
}

func (c *checker) pathsToType(paths []pathType) super.Type {
	fuser := c.newFuser()
	for _, path := range paths {
		fuser.fuse(c.pathToRec(path.typ, path.elems))
	}
	return fuser.Type(c)
}

func (c *checker) pathToRec(typ super.Type, elems []string) super.Type {
	for ; len(elems) > 0; elems = elems[:len(elems)-1] {
		last := elems[len(elems)-1]
		typ = c.sctx.MustLookupTypeRecord([]super.Field{{Name: last, Type: typ}})
	}
	return typ
}

func (c *checker) dropPaths(typ super.Type, drops []path) super.Type {
	for _, drop := range drops {
		typ = c.dropPath(typ, drop)
	}
	return typ
}

func (c *checker) dropPath(typ super.Type, drop path) super.Type {
	if len(drop.elems) == 0 {
		return nil
	}
	// Drop is a little tricky since it passes through non-record values so
	// we need to preserve any union type presented to its input. pickRec returns
	// a copy of the types slice so we can modify it.
	types, pick := pickRec(typ)
	if types == nil {
		// drop passes through non-records
		return typ
	}
	rec := super.TypeUnder(types[pick]).(*super.TypeRecord)
	off, ok := rec.IndexOfField(drop.elems[0])
	if !ok {
		c.error(drop.loc, fmt.Errorf("no such field to drop: %q", drop.elems[0]))
		return c.unknown
	}
	fields := slices.Clone(rec.Fields)
	childType := c.dropPath(fields[off].Type, path{drop.loc, drop.elems[1:]})
	if childType == nil {
		fields = slices.Delete(fields, off, off+1)
	} else {
		fields[off].Type = childType
	}
	types[pick] = c.sctx.MustLookupTypeRecord(fields)
	if len(types) > 1 {
		return c.sctx.LookupTypeUnion(types)
	}
	return types[0]
}

func pickRec(typ super.Type) ([]super.Type, int) {
	switch typ := super.TypeUnder(typ).(type) {
	case *super.TypeRecord:
		return []super.Type{typ}, 0
	case *super.TypeUnion:
		types := slices.Clone(typ.Types)
		for k := range types {
			if _, ok := super.TypeUnder(types[k]).(*super.TypeRecord); ok {
				return types, k
			}
		}
	}
	return nil, 0
}

func (c *checker) putPaths(typ super.Type, puts []pathType) super.Type {
	// Fuse each path as a single-record path into the input type.
	fuser := c.newFuser()
	fuser.fuse(typ)
	for _, put := range puts {
		fuser.fuse(c.pathToRec(put.typ, put.elems))
	}
	return fuser.Type(c)
}

type path struct {
	loc   ast.Node
	elems []string
}

func (c *checker) lvalsToPaths(exprs []sem.Expr) []path {
	var paths []path
	for _, e := range exprs {
		this, ok := e.(*sem.ThisExpr)
		if !ok {
			return nil
		}
		paths = append(paths, path{loc: this.Node, elems: this.Path})
	}
	return paths
}

func (c *checker) fuse(types []super.Type) super.Type {
	if len(types) == 0 {
		return c.unknown
	}
	if len(types) == 1 {
		return types[0]
	}
	fuser := c.newFuser()
	for _, typ := range types {
		fuser.fuse(typ)
	}
	return fuser.Type(c)
}

func (c *checker) boolean(loc ast.Node, typ super.Type) bool {
	ok := typeCheck(typ, func(typ super.Type) bool {
		return typ == super.TypeBool || typ == super.TypeNull
	})
	if !ok {
		c.error(loc, fmt.Errorf("boolean type required, encountered type %q", sup.FormatType(typ)))
	}
	return ok
}

func typeCheck(typ super.Type, check func(super.Type) bool) bool {
	if isUnknown(typ) {
		return true
	}
	if u, ok := super.TypeUnder(typ).(*super.TypeUnion); ok {
		for _, t := range u.Types {
			if typeCheck(t, check) {
				return true
			}
		}
		return false
	}
	return check(typ)
}

func (c *checker) integer(loc ast.Node, typ super.Type) bool {
	ok := typeCheck(typ, func(typ super.Type) bool {
		return super.IsInteger(typ.ID())
	})
	if !ok {
		c.error(loc, fmt.Errorf("integer type required, encountered %s", sup.FormatType(typ)))
	}
	return ok
}

func (c *checker) number(loc ast.Node, typ super.Type) bool {
	ok := typeCheck(typ, func(typ super.Type) bool {
		id := typ.ID()
		return super.IsNumber(id) || id == super.IDNull
	})
	if !ok {
		c.error(loc, fmt.Errorf("numeric type required, encountered %s", sup.FormatType(typ)))
	}
	return ok
}

// XXX need to also prop unions for deref on multiple types, e.g., map keys as strings + records

func (c *checker) deref(loc ast.Node, typ super.Type, field string) (super.Type, bool) {
	switch typ := super.TypeUnder(typ).(type) {
	case *super.TypeOfNull:
		return super.TypeNull, true //XXX add tests for this
	case *super.TypeError:
		if isUnknown(typ) {
			return typ, true
		}
	case *super.TypeMap:
		return c.indexMap(loc, typ, super.TypeString)
	case *super.TypeRecord:
		which, ok := typ.IndexOfField(field)
		if !ok {
			if !hasAny(typ) {
				c.error(loc, fmt.Errorf("%q no such field", field))
			}
			return c.unknown, false
		}
		return typ.Fields[which].Type, true
	case *super.TypeUnion:
		// Push the error stack and if we find some valid deref,
		// we'll discard the errors.  Otherwise, we'll keep them.
		c.epush()
		var types []super.Type
		var valid bool
		for _, t := range typ.Types {
			typ, ok := c.deref(loc, t, field)
			if ok {
				types = append(types, typ)
				valid = true
			}
		}
		errs := c.epop()
		if !valid {
			c.ekeep(errs)
		}
		return c.fuse(types), valid
	}
	c.error(loc, fmt.Errorf("%q no such field", field))
	return c.unknown, false
}

func (c *checker) logical(lloc, rloc ast.Node, lhs, rhs super.Type) {
	c.boolean(lloc, lhs)
	c.boolean(rloc, rhs)
}

func (c *checker) in(lloc, rloc ast.Node, lhs, rhs super.Type) {
	if hasAny(lhs) || hasAny(rhs) {
		//XXX should still check that RHS is container
		return
	}
	//XXX need to walk unions, then how to combine... just need to find one that works
	switch typ := super.TypeUnder(rhs).(type) {
	case *super.TypeOfNull:
	case *super.TypeArray:
		//XXX check that lhs can be in the array
		if !comparable(lhs, typ.Type) {
			//XXX better error message
			c.error(lloc, errors.New("left-hand side of in operator not type-compatible with right-hand side"))
		}
	case *super.TypeSet:
		//XXX check that lhs can be in the array
		if !comparable(lhs, typ.Type) {
			//XXX better error message
			c.error(lloc, errors.New("left-hand side of in operator not type compatible with right-hand side"))
		}
	case *super.TypeRecord:
		// XXX
	case *super.TypeMap:
		// XXX
	case *super.TypeOfNet:
		// XXX
	default:
		c.error(rloc, fmt.Errorf("in-operator bad type")) //XXX
	}
}

func (c *checker) equality(lhs, rhs super.Type) super.Type {
	comparable(lhs, rhs)
	return super.TypeBool
}

func (c *checker) comparison(lhs, rhs super.Type) super.Type {
	comparable(lhs, rhs)
	return super.TypeBool
}

func comparable(a, b super.Type) bool {
	aid := super.TypeUnder(a).ID() //XXX
	bid := super.TypeUnder(b).ID()
	if aid == bid || aid == super.IDNull || bid == super.IDNull {
		return true
	}
	if super.IsNumber(aid) {
		return super.IsNumber(bid)
	}
	switch super.TypeUnder(a).(type) {
	case *super.TypeRecord:
		_, ok := super.TypeUnder(b).(*super.TypeRecord)
		return ok
	case *super.TypeArray:
		if _, ok := super.TypeUnder(b).(*super.TypeArray); ok {
			return ok
		}
		_, ok := super.TypeUnder(b).(*super.TypeSet)
		return ok
	case *super.TypeSet:
		if _, ok := super.TypeUnder(b).(*super.TypeArray); ok {
			return ok
		}
		_, ok := super.TypeUnder(b).(*super.TypeSet)
		return ok
	case *super.TypeMap:
		_, ok := super.TypeUnder(b).(*super.TypeMap)
		return ok
	}
	return false
}

func (c *checker) arithmetic(lloc, rloc ast.Node, lhs, rhs super.Type) super.Type {
	if isUnknown(lhs) || isUnknown(rhs) {
		return c.unknown
	}
	c.number(lloc, lhs)
	c.number(rloc, rhs)
	//XXX coerce?
	return c.fuse([]super.Type{lhs, rhs})
}

func (c *checker) plus(lloc, rloc ast.Node, lhs, rhs super.Type) super.Type {
	if isUnknown(lhs) || isUnknown(rhs) {
		return c.unknown
	}
	if hasString(lhs) && hasString(rhs) {
		return c.fuse([]super.Type{lhs, rhs})
	}
	if hasNumber(lhs) && hasNumber(rhs) {
		return c.fuse([]super.Type{lhs, rhs})
	}
	c.error(lloc, errors.New("type mismatch"))
	return c.unknown
}

func hasNumber(typ super.Type) bool {
	id := super.TypeUnder(typ).ID()
	if super.IsNumber(id) || id == super.IDNull {
		return true
	}
	if u, ok := super.TypeUnder(typ).(*super.TypeUnion); ok {
		for _, t := range u.Types {
			if hasNumber(t) {
				return true
			}
		}
	}
	return false
}

func hasString(typ super.Type) bool {
	switch typ := super.TypeUnder(typ).(type) {
	case *super.TypeOfString, *super.TypeOfNull:
		return true
	case *super.TypeUnion:
		for _, t := range typ.Types {
			if hasString(t) {
				return true
			}
		}
	}
	return false
}

func isUnknown(typ super.Type) bool {
	if err, ok := super.TypeUnder(typ).(*super.TypeError); ok {
		if rec, ok := err.Type.(*super.TypeRecord); ok {
			return len(rec.Fields) == 0
		}
	}
	return false
}

func hasAny(typ super.Type) bool {
	if u, ok := super.TypeUnder(typ).(*super.TypeUnion); ok {
		for _, t := range u.Types {
			if hasAny(t) {
				return true
			}
		}
	}
	return isUnknown(typ)
}

func (c *checker) indexOf(cloc, iloc ast.Node, container, index super.Type) super.Type {
	//XXX need to walk unions, then how to combine... just need to find one that works
	if hasAny(container) {
		return c.unknown
	}
	switch typ := super.TypeUnder(container).(type) {
	case *super.TypeArray:
		c.integer(iloc, index)
		return typ.Type
	case *super.TypeSet:
		c.integer(iloc, index)
		return typ.Type
	case *super.TypeRecord:
		ok := typeCheck(index, func(typ super.Type) bool {
			id := super.TypeUnder(typ).ID()
			return id == super.IDString || super.IsInteger(id) || id == super.IDNull
		})
		if !ok {
			c.error(iloc, errors.New("string or integer type required to index record"))
		}
		return c.unknown //XXX
	case *super.TypeMap:
		return c.unknown //XXX
	default:
		c.error(cloc, fmt.Errorf("indexed entity is not indexable"))
		return c.unknown //XXX
	}
}

func (c *checker) indexMap(loc ast.Node, m *super.TypeMap, index super.Type) (super.Type, bool) {
	if isUnknown(index) {
		return c.unknown, true
	}
	if err := c.coerceable(index, m.KeyType); err != nil {
		c.error(loc, err)
		return c.unknown, false
	}
	return m.ValType, true
}

func (c *checker) sliceable(loc ast.Node, typ super.Type) {
	if hasAny(typ) {
		return
	}
	switch super.TypeUnder(typ).(type) {
	case *super.TypeArray, *super.TypeSet, *super.TypeRecord:
	default:
		c.error(loc, fmt.Errorf("sliced entity is not sliceable"))
	}
}

func coercable(from, to super.Type) bool {
	fromID := super.TypeUnder(from).ID() //XXX
	toID := super.TypeUnder(to).ID()
	if fromID == toID || aid == super.IDNull || bid == super.IDNull {
		return true
	}
	if super.IsNumber(aid) {
		return super.IsNumber(bid)
	}
	switch super.TypeUnder(a).(type) {
	case *super.TypeRecord:
		_, ok := super.TypeUnder(b).(*super.TypeRecord)
		return ok
	case *super.TypeArray:
		if _, ok := super.TypeUnder(b).(*super.TypeArray); ok {
			return ok
		}
		_, ok := super.TypeUnder(b).(*super.TypeSet)
		return ok
	case *super.TypeSet:
		if _, ok := super.TypeUnder(b).(*super.TypeArray); ok {
			return ok
		}
		_, ok := super.TypeUnder(b).(*super.TypeSet)
		return ok
	case *super.TypeMap:
		_, ok := super.TypeUnder(b).(*super.TypeMap)
		return ok
	}
	return false
}

func (c *checker) epush() {
	c.estack = append(c.estack, nil)
}

func (c *checker) epop() errlist {
	n := len(c.estack) - 1
	errs := c.estack[n]
	c.estack = c.estack[:n]
	return errs
}

func (c *checker) ekeep(errs errlist) {
	n := len(c.estack) - 1
	c.estack[n] = append(c.estack[n], errs...)
}

func (c *checker) error(loc ast.Node, err error) {
	c.estack[len(c.estack)-1].error(loc, err)
}

func (c *checker) newFuser() *fuser {
	return &fuser{sctx: c.sctx}
}

type fuser struct {
	sctx *super.Context
	typ  super.Type
	sch  *agg.Schema
}

func (f *fuser) fuse(typ super.Type) {
	if f.sch != nil {
		f.sch.Mixin(typ)
	} else if f.typ == nil {
		f.typ = typ
	} else if f.typ != typ {
		f.sch = agg.NewSchema(f.sctx)
		f.sch.Mixin(f.typ)
		f.sch.Mixin(typ)
	}
}

func (f *fuser) Type(c *checker) super.Type {
	if f.sch != nil {
		return f.sch.Type()
	}
	if f.typ != nil {
		return f.typ
	}
	return c.unknown
}
