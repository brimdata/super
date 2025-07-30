package semantic

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/rungen"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/plural"
	"github.com/brimdata/super/pkg/reglob"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/segmentio/ksuid"
)

func (a *analyzer) semSeq(seq ast.Seq) dag.Seq {
	var converted dag.Seq
	for k, op := range seq {
		if d, ok := op.(*ast.Debug); ok {
			return a.semDebugOp(d, seq[k+1:], converted)
		}
		converted = a.semOp(op, converted)
	}
	return converted
}

func (a *analyzer) semFrom(from *ast.From, seq dag.Seq) (dag.Seq, schema) {
	if len(from.Elems) > 1 {
		a.error(from, errors.New("cross join implied by multiple elements in from clause is not yet supported"))
		return dag.Seq{badOp()}, badSchema()
	}
	return a.semFromElem(from.Elems[0], seq)
}

// semFromElem generates a DAG fragment to read from the various sources potentially
// with embedded SQL subexpressions and joins.  We return the schema of the
// the entity to support SQL scoping semantics.  The callee is responsible for
// wrapping the result in a record representing the schemafied data as the output
// here is simply the underlying data sequence.
func (a *analyzer) semFromElem(elem *ast.FromElem, seq dag.Seq) (dag.Seq, schema) {
	var sch schema
	seq, sch = a.semFromEntity(elem.Entity, elem.Alias, elem.Args, seq)
	if elem.Ordinality != nil {
		a.error(elem.Ordinality, errors.New("WITH ORDINALITY clause is not yet supported"))
		return dag.Seq{badOp()}, badSchema()
	}
	return seq, sch
}

func (a *analyzer) fromSchema(alias *ast.TableAlias, name string) schema {
	if alias != nil {
		name = alias.Name
		if len(alias.Columns) != 0 {
			a.error(alias, errors.New("cannot apply aliased columns to dynamically typed data"))
		}
	}
	return &dynamicSchema{name: name}
}

func (a *analyzer) semFromEntity(entity ast.FromEntity, alias *ast.TableAlias, args []ast.OpArg, seq dag.Seq) (dag.Seq, schema) {
	switch entity := entity.(type) {
	case *ast.Glob:
		if bad := a.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		s := a.fromSchema(alias, "")
		if a.env.IsLake() {
			return a.semPoolFromRegexp(entity, reglob.Reglob(entity.Pattern), entity.Pattern, "glob", args), s
		}
		return dag.Seq{a.semFromFileGlob(entity, entity.Pattern, args)}, s
	case *ast.Regexp:
		if bad := a.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		if !a.env.IsLake() {
			a.error(entity, errors.New("cannot use regular expression with from operator on local file system"))
			return seq, badSchema()
		}
		return a.semPoolFromRegexp(entity, entity.Pattern, entity.Pattern, "regexp", args), a.fromSchema(alias, "")
	case *ast.Text:
		if bad := a.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		if c, ok := a.scope.ctes[strings.ToLower(entity.Text)]; ok {
			seq, schema, err := derefSchemaWithAlias(c.schema, alias, dag.CopySeq(c.body))
			if err != nil {
				a.error(alias, err)
			}
			return seq, schema
		}
		op, def := a.semFromName(entity, entity.Text, args)
		return dag.Seq{op}, a.fromSchema(alias, def)
	case *ast.ExprEntity:
		seq, def := a.semFromExpr(entity, args, seq)
		return seq, a.fromSchema(alias, def)
	case *ast.LakeMeta:
		if bad := a.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		return dag.Seq{a.semLakeMeta(entity)}, &dynamicSchema{}
	case *ast.SQLPipe:
		return a.semSQLPipe(entity, seq, alias)
	case *ast.SQLJoin:
		return a.semSQLJoin(entity, seq)
	case *ast.SQLCrossJoin:
		return a.semCrossJoin(entity, seq)
	default:
		panic(fmt.Sprintf("semFromEntity: unknown entity type: %T", entity))
	}
}

func (a *analyzer) semFromExpr(entity *ast.ExprEntity, args []ast.OpArg, seq dag.Seq) (dag.Seq, string) {
	expr := a.semExpr(entity.Expr)
	val, err := rungen.EvalAtCompileTime(a.sctx, expr)
	if err == nil && !hasError(val) {
		if bad := a.hasFromParent(entity, seq); bad != nil {
			return bad, ""
		}
		return a.semFromConstVal(val, entity, args)
	}
	// This is an expression so set up a robot scanner that pulls values from
	// parent to decide what to scan.
	return append(seq, &dag.RobotScan{
		Kind:   "RobotScan",
		Expr:   expr,
		Format: a.asFormatArg(args),
	}), ""
}

func hasError(val super.Value) bool {
	has := function.NewHasError()
	result := has.Call([]super.Value{val})
	return result.AsBool()
}

func (a *analyzer) hasFromParent(loc ast.Node, seq dag.Seq) dag.Seq {
	if len(seq) > 0 {
		a.error(loc, errors.New("from operator cannot have parent unless from argument is an expression"))
		return append(seq, badOp())
	}
	return nil
}

func (a *analyzer) semFromConstVal(val super.Value, entity *ast.ExprEntity, args []ast.OpArg) (dag.Seq, string) {
	if super.TypeUnder(val.Type()) == super.TypeString {
		op, name := a.semFromName(entity, val.AsString(), args)
		return dag.Seq{op}, name
	}
	vals, err := val.Elements()
	if err != nil {
		a.error(entity.Expr, fmt.Errorf("from expression requires a string but encountered %s", sup.String(val)))
		return dag.Seq{badOp()}, ""
	}
	names := make([]string, 0, len(vals))
	for _, val := range vals {
		if super.TypeUnder(val.Type()) != super.TypeString {
			a.error(entity.Expr, fmt.Errorf("from expression requires a string but encountered %s", sup.String(val)))
			return dag.Seq{badOp()}, ""
		}
		names = append(names, val.AsString())
	}
	if len(names) == 1 {
		op, _ := a.semFromName(entity, names[0], args)
		return dag.Seq{op}, names[0]
	}
	var paths []dag.Seq
	for _, name := range names {
		op, _ := a.semFromName(entity, name, args)
		paths = append(paths, dag.Seq{op})
	}
	return dag.Seq{
		&dag.Fork{
			Kind:  "Fork",
			Paths: paths,
		},
	}, ""
}

func (a *analyzer) semFromName(nameLoc ast.Node, name string, args []ast.OpArg) (dag.Op, string) {
	if isURL(name) {
		return a.semFromURL(nameLoc, name, args), ""
	}
	prefix := strings.Split(name, ".")[0]
	if a.env.IsLake() {
		return a.semPool(nameLoc, name, args), prefix
	}
	return a.semFile(name, args), prefix
}

func (a *analyzer) asFormatArg(args []ast.OpArg) string {
	opArgs := a.semOpArgs(args, "format")
	s, _ := a.textArg(opArgs, "format")
	return s
}

func (a *analyzer) semFile(name string, args []ast.OpArg) dag.Op {
	format := a.asFormatArg(args)
	if format == "" {
		format = zio.FormatFromPath(name)
	}
	return &dag.FileScan{
		Kind:   "FileScan",
		Path:   name,
		Format: format,
	}
}

func (a *analyzer) semFromFileGlob(globLoc ast.Node, pattern string, args []ast.OpArg) dag.Op {
	names, err := filepath.Glob(pattern)
	if err != nil {
		a.error(globLoc, err)
		return badOp()
	}
	if len(names) == 0 {
		a.error(globLoc, errors.New("no file names match glob pattern"))
		return badOp()
	}
	if len(names) == 1 {
		return a.semFile(names[0], args)
	}
	paths := make([]dag.Seq, 0, len(names))
	for _, name := range names {
		paths = append(paths, dag.Seq{a.semFile(name, args)})
	}
	return &dag.Fork{
		Kind:  "Fork",
		Paths: paths,
	}
}

func (a *analyzer) semFromURL(urlLoc ast.Node, u string, args []ast.OpArg) dag.Op {
	_, err := url.ParseRequestURI(u)
	if err != nil {
		a.error(urlLoc, err)
		return badOp()
	}
	opArgs := a.semOpArgs(args, "format", "method", "body", "headers")
	format, _ := a.textArg(opArgs, "format")
	method, _ := a.textArg(opArgs, "method")
	body, _ := a.textArg(opArgs, "body")
	var headers map[string][]string
	if e, loc := a.exprArg(opArgs, "headers"); e != nil {
		val, err := rungen.EvalAtCompileTime(a.sctx, e)
		if err != nil {
			a.error(loc, err)
		} else {
			headers, err = unmarshalHeaders(val)
			if err != nil {
				a.error(loc, err)
			}
		}
	}
	if format == "" {
		format = zio.FormatFromPath(u)
	}
	return &dag.HTTPScan{
		Kind:    "HTTPScan",
		URL:     u,
		Format:  format,
		Method:  method,
		Headers: headers,
		Body:    body,
	}
}

func unmarshalHeaders(val super.Value) (map[string][]string, error) {
	if !super.IsRecordType(val.Type()) {
		return nil, errors.New("headers value must be a record")
	}
	headers := map[string][]string{}
	for i, f := range val.Fields() {
		if inner := super.InnerType(f.Type); inner == nil || inner.ID() != super.IDString {
			return nil, errors.New("headers field value must be an array or set of strings")
		}
		fieldVal := val.DerefByColumn(i)
		if fieldVal == nil {
			continue
		}
		for it := fieldVal.Iter(); !it.Done(); {
			if b := it.Next(); b != nil {
				headers[f.Name] = append(headers[f.Name], super.DecodeString(b))
			}
		}
	}
	return headers, nil
}

func (a *analyzer) semPoolFromRegexp(patternLoc ast.Node, re, orig, which string, args []ast.OpArg) dag.Seq {
	poolNames, err := a.matchPools(re, orig, which)
	if err != nil {
		a.error(patternLoc, err)
		return dag.Seq{badOp()}
	}
	var paths []dag.Seq
	for _, name := range poolNames {
		paths = append(paths, dag.Seq{a.semPool(patternLoc, name, args)})
	}
	return dag.Seq{&dag.Fork{
		Kind:  "Fork",
		Paths: paths,
	}}
}

func (a *analyzer) semSortExpr(sch schema, s ast.SortExpr, reverse bool) dag.SortExpr {
	var e dag.Expr
	if sch != nil {
		e = a.semExprSchema(sch, s.Expr)
		if which, ok := isOrdinal(a.sctx, e); ok {
			var err error
			if e, err = sch.resolveOrdinal(which); err != nil {
				a.error(s.Expr, err)
			}
		}
	} else {
		e = a.semExpr(s.Expr)
	}
	o := order.Asc
	if s.Order != nil {
		var err error
		if o, err = order.Parse(s.Order.Name); err != nil {
			a.error(s.Order, err)
		}
	}
	if reverse {
		o = !o
	}
	n := order.NullsLast
	if s.Nulls != nil {
		if err := n.UnmarshalText([]byte(s.Nulls.Name)); err != nil {
			a.error(s.Nulls, err)
		}
	}
	return dag.SortExpr{Key: e, Order: o, Nulls: n}
}

func (a *analyzer) semPool(nameLoc ast.Node, poolName string, args []ast.OpArg) dag.Op {
	opArgs := a.semOpArgs(args, "commit", "meta", "tap")
	poolID, err := a.env.PoolID(a.ctx, poolName)
	if err != nil {
		a.error(nameLoc, err)
		return badOp()
	}
	var commitID ksuid.KSUID
	commit, commitLoc := a.textArg(opArgs, "commit")
	if commit != "" {
		if commitID, err = lakeparse.ParseID(commit); err != nil {
			commitID, err = a.env.CommitObject(a.ctx, poolID, commit)
			if err != nil {
				a.error(commitLoc, err)
				return badOp()
			}
		}
	}
	meta, metaLoc := a.textArg(opArgs, "meta")
	if meta != "" {
		if _, ok := dag.CommitMetas[meta]; ok {
			if commitID == ksuid.Nil {
				commitID, err = a.env.CommitObject(a.ctx, poolID, "main")
				if err != nil {
					a.error(metaLoc, err)
					return badOp()
				}
			}
			tapString, _ := a.textArg(opArgs, "tap")
			tap := tapString != ""
			return &dag.CommitMetaScan{
				Kind:   "CommitMetaScan",
				Meta:   meta,
				Pool:   poolID,
				Commit: commitID,
				Tap:    tap,
			}
		}
		if _, ok := dag.PoolMetas[meta]; ok {
			return &dag.PoolMetaScan{
				Kind: "PoolMetaScan",
				Meta: meta,
				ID:   poolID,
			}
		}
		a.error(nameLoc, fmt.Errorf("unknown metadata type %q", meta))
		return badOp()
	}
	if commitID == ksuid.Nil {
		// This trick here allows us to default to the main branch when
		// there is a "from pool" operator with no meta query or commit object.
		commitID, err = a.env.CommitObject(a.ctx, poolID, "main")
		if err != nil {
			a.error(nameLoc, err)
			return badOp()
		}
	}
	return &dag.PoolScan{
		Kind:   "PoolScan",
		ID:     poolID,
		Commit: commitID,
	}
}

func (a *analyzer) semLakeMeta(entity *ast.LakeMeta) dag.Op {
	meta := entity.Meta.Text
	if _, ok := dag.LakeMetas[meta]; !ok {
		a.error(entity, fmt.Errorf("unknown lake metadata type %q in from operator", meta))
		return badOp()
	}
	return &dag.LakeMetaScan{
		Kind: "LakeMetaScan",
		Meta: meta,
	}
}

func (a *analyzer) semDelete(op *ast.Delete) dag.Op {
	if !a.env.IsLake() {
		a.error(op, errors.New("deletion requires data lake"))
		return badOp()
	}
	poolID, err := a.env.PoolID(a.ctx, op.Pool)
	if err != nil {
		a.error(op, err)
		return badOp()
	}
	var commitID ksuid.KSUID
	if op.Branch != "" {
		var err error
		if commitID, err = lakeparse.ParseID(op.Branch); err != nil {
			commitID, err = a.env.CommitObject(a.ctx, poolID, op.Branch)
			if err != nil {
				a.error(op, err)
				return badOp()
			}
		}
	}
	return &dag.DeleteScan{
		Kind:   "DeleteScan",
		ID:     poolID,
		Commit: commitID,
	}
}

func (a *analyzer) matchPools(pattern, origPattern, patternDesc string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	pools, err := a.env.Lake().ListPools(a.ctx)
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, p := range pools {
		if re.MatchString(p.Name) {
			matches = append(matches, p.Name)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s: pool matching %s not found", origPattern, patternDesc)
	}
	return matches, nil
}

func (a *analyzer) semScope(op *ast.Scope) *dag.Scope {
	a.scope = NewScope(a.scope)
	defer a.exitScope()
	consts, funcs := a.semDecls(op.Decls)
	return &dag.Scope{
		Kind:   "Scope",
		Consts: consts,
		Funcs:  funcs,
		Body:   a.semSeq(op.Body),
	}
}

func (a *analyzer) semDebugOp(o *ast.Debug, mainAst ast.Seq, in dag.Seq) dag.Seq {
	output := &dag.Output{Kind: "Output", Name: "debug"}
	a.outputs[output] = o
	e := a.semExprNullable(o.Expr)
	if e == nil {
		e = &dag.This{Kind: "This"}
	}
	y := &dag.Values{Kind: "Values", Exprs: []dag.Expr{e}}
	main := a.semSeq(mainAst)
	if len(main) == 0 {
		main.Append(&dag.Pass{Kind: "Pass"})
	}
	return append(in, &dag.Mirror{
		Kind:   "Mirror",
		Main:   main,
		Mirror: dag.Seq{y, output},
	})
}

// semOp does a semantic analysis on a flowgraph to an
// intermediate representation that can be compiled into the runtime
// object.  Currently, it only replaces the aggregate duration with
// a bucket call on the ts and replaces FunctionCalls in op context
// with either an aggregate or filter op based on the function's name.
func (a *analyzer) semOp(o ast.Op, seq dag.Seq) dag.Seq {
	switch o := o.(type) {
	case *ast.SQLSelect, *ast.SQLLimitOffset, *ast.SQLOrderBy, *ast.SQLPipe, *ast.SQLUnion, *ast.SQLWith, *ast.SQLValues:
		seq, sch := a.semSQLOp(o, seq)
		seq, _ = derefSchema(sch, seq)
		return seq
	case *ast.From:
		seq, _ := a.semFrom(o, seq)
		return seq
	case *ast.DefaultScan:
		return append(seq, &dag.DefaultScan{Kind: "DefaultScan"})
	case *ast.Delete:
		if len(seq) > 0 {
			panic("analyzer.SemOp: delete scan cannot have parent in AST")
		}
		return dag.Seq{a.semDelete(o)}
	case *ast.Aggregate:
		keys := a.semAssignments(o.Keys)
		a.checkStaticAssignment(o.Keys, keys)
		if len(keys) == 0 && len(o.Aggs) == 1 {
			if seq := a.singletonAgg(o.Aggs[0], seq); seq != nil {
				return seq
			}
		}
		aggs := a.semAssignments(o.Aggs)
		a.checkStaticAssignment(o.Aggs, aggs)
		// Note: InputSortDir is copied in here but it's not meaningful
		// coming from a parser AST, only from a worker using the DAG,
		// which is another reason why we need separate AST and DAG.
		// Said another way, we don't want to do semantic analysis on a worker AST
		// as we presume that work had already been done and we just need
		// to execute it.  For now, the worker only uses a filter expression
		// so this code path isn't hit yet, but it uses this same entry point
		// and it will soon do other stuff so we need to put in place the
		// separation... see issue #2163.
		return append(seq, &dag.Aggregate{
			Kind:  "Aggregate",
			Limit: o.Limit,
			Keys:  keys,
			Aggs:  aggs,
		})
	case *ast.Parallel:
		var paths []dag.Seq
		for _, seq := range o.Paths {
			paths = append(paths, a.semSeq(seq))
		}
		return append(seq, &dag.Fork{
			Kind:  "Fork",
			Paths: paths,
		})
	case *ast.Scope:
		return append(seq, a.semScope(o))
	case *ast.Switch:
		var expr dag.Expr
		if o.Expr != nil {
			expr = a.semExpr(o.Expr)
		}
		var cases []dag.Case
		for _, c := range o.Cases {
			var e dag.Expr
			if c.Expr != nil {
				e = a.semExpr(c.Expr)
			} else if o.Expr == nil {
				// c.Expr == nil indicates the default case,
				// whose handling depends on p.Expr.
				e = &dag.Literal{
					Kind:  "Literal",
					Value: "true",
				}
			}
			path := a.semSeq(c.Path)
			cases = append(cases, dag.Case{Expr: e, Path: path})
		}
		return append(seq, &dag.Switch{
			Kind:  "Switch",
			Expr:  expr,
			Cases: cases,
		})
	case *ast.Shape:
		return append(seq, &dag.Shape{Kind: "Shape"})
	case *ast.Cut:
		assignments := a.semAssignments(o.Args)
		// Collect static paths so we can check on what is available.
		var fields field.List
		for _, a := range assignments {
			if this, ok := a.LHS.(*dag.This); ok {
				fields = append(fields, this.Path)
			}
		}
		if _, err := super.NewRecordBuilder(a.sctx, fields); err != nil {
			a.error(o.Args, err)
			return append(seq, badOp())
		}
		return append(seq, &dag.Cut{
			Kind: "Cut",
			Args: assignments,
		})
	case *ast.Distinct:
		return append(seq, &dag.Distinct{
			Kind: "Distinct",
			Expr: a.semExpr(o.Expr),
		})
	case *ast.Drop:
		args := a.semFields(o.Args)
		if len(args) == 0 {
			a.error(o, errors.New("no fields given"))
		}
		return append(seq, &dag.Drop{
			Kind: "Drop",
			Args: args,
		})
	case *ast.Sort:
		var sortExprs []dag.SortExpr
		for _, e := range o.Exprs {
			sortExprs = append(sortExprs, a.semSortExpr(nil, e, o.Reverse))
		}
		return append(seq, &dag.Sort{
			Kind:    "Sort",
			Exprs:   sortExprs,
			Reverse: o.Reverse && len(sortExprs) == 0,
		})
	case *ast.Head:
		count := 1
		if o.Count != nil {
			count = a.evalPositiveInteger(o.Count)
		}
		return append(seq, &dag.Head{
			Kind:  "Head",
			Count: count,
		})
	case *ast.Tail:
		count := 1
		if o.Count != nil {
			count = a.evalPositiveInteger(o.Count)
		}
		return append(seq, &dag.Tail{
			Kind:  "Tail",
			Count: count,
		})
	case *ast.Skip:
		return append(seq, &dag.Skip{
			Kind:  "Skip",
			Count: a.evalPositiveInteger(o.Count),
		})
	case *ast.Uniq:
		return append(seq, &dag.Uniq{
			Kind:  "Uniq",
			Cflag: o.Cflag,
		})
	case *ast.Pass:
		return append(seq, dag.PassOp)
	case *ast.OpExpr:
		return a.semOpExpr(o.Expr, seq)
	case *ast.Search:
		e := a.semExpr(o.Expr)
		return append(seq, dag.NewFilter(e))
	case *ast.Where:
		e := a.semExpr(o.Expr)
		return append(seq, dag.NewFilter(e))
	case *ast.Top:
		limit := 1
		if o.Limit != nil {
			l := a.semExpr(o.Limit)
			val, err := rungen.EvalAtCompileTime(a.sctx, l)
			if err != nil {
				a.error(o.Limit, err)
				return append(seq, badOp())
			}
			if !super.IsSigned(val.Type().ID()) {
				a.error(o.Limit, errors.New("limit argument must be an integer"))
				return append(seq, badOp())
			}
			if limit = int(val.Int()); limit < 1 {
				a.error(o.Limit, errors.New("limit argument value must be greater than 0"))
				return append(seq, badOp())
			}
		}
		var exprs []dag.SortExpr
		for _, e := range o.Exprs {
			exprs = append(exprs, a.semSortExpr(nil, e, o.Reverse))
		}
		return append(seq, &dag.Top{
			Kind:    "Top",
			Limit:   limit,
			Exprs:   exprs,
			Reverse: o.Reverse && len(exprs) == 0,
		})
	case *ast.Put:
		assignments := a.semAssignments(o.Args)
		// We can do collision checking on static paths, so check what we can.
		var fields field.List
		for _, a := range assignments {
			if this, ok := a.LHS.(*dag.This); ok {
				fields = append(fields, this.Path)
			}
		}
		if err := expr.CheckPutFields(fields); err != nil {
			a.error(o, err)
		}
		return append(seq, &dag.Put{
			Kind: "Put",
			Args: assignments,
		})
	case *ast.OpAssignment:
		return append(seq, a.semOpAssignment(o))
	case *ast.Rename:
		var assignments []dag.Assignment
		for _, fa := range o.Args {
			assign := a.semAssignment(fa)
			if !isLval(assign.RHS) {
				a.error(fa.RHS, fmt.Errorf("illegal right-hand side of assignment"))
			}
			// If both paths are static validate them. Otherwise this will be
			// done at runtime.
			lhs, lhsOk := assign.LHS.(*dag.This)
			rhs, rhsOk := assign.RHS.(*dag.This)
			if rhsOk && lhsOk {
				if err := expr.CheckRenameField(lhs.Path, rhs.Path); err != nil {
					a.error(&fa, err)
				}
			}
			assignments = append(assignments, assign)
		}
		return append(seq, &dag.Rename{
			Kind: "Rename",
			Args: assignments,
		})
	case *ast.Fuse:
		return append(seq, &dag.Fuse{Kind: "Fuse"})
	case *ast.Join:
		leftAlias, rightAlias := "left", "right"
		if o.Alias != nil {
			leftAlias = o.Alias.Left.Name
			rightAlias = o.Alias.Right.Name
		}
		if leftAlias == rightAlias {
			a.error(o.Alias, errors.New("left and right join aliases cannot be the same"))
			return append(seq, badOp())
		}
		leftKey, rightKey, err := a.semJoinCond(o.Cond, leftAlias, rightAlias)
		if err != nil {
			a.error(o.Cond, err)
			return append(seq, badOp())
		}
		style := o.Style
		if style == "" {
			style = "inner"
		}
		join := &dag.Join{
			Kind:       "Join",
			Style:      style,
			LeftAlias:  leftAlias,
			LeftDir:    order.Unknown,
			LeftKey:    leftKey,
			RightAlias: rightAlias,
			RightDir:   order.Unknown,
			RightKey:   rightKey,
		}
		if o.RightInput == nil {
			return append(seq, join)
		}
		if len(seq) == 0 {
			seq = append(seq, dag.PassOp)
		}
		fork := &dag.Fork{
			Kind: "Fork",
			Paths: []dag.Seq{
				seq,
				a.semSeq(o.RightInput),
			},
		}
		return dag.Seq{fork, join}
	case *ast.Explode:
		typ, err := a.semType(o.Type)
		if err != nil {
			a.error(o.Type, err)
			typ = "<bad type expr>"
		}
		args := a.semExprs(o.Args)
		var as string
		if o.As == nil {
			as = "value"
		} else {
			e := a.semExpr(o.As)
			this, ok := e.(*dag.This)
			if !ok {
				a.error(o.As, errors.New("as clause must be a field reference"))
				return append(seq, badOp())
			} else if len(this.Path) != 1 {
				a.error(o.As, errors.New("field must be a top-level field"))
				return append(seq, badOp())
			}
			as = this.Path[0]
		}
		return append(seq, &dag.Explode{
			Kind: "Explode",
			Args: args,
			Type: typ,
			As:   as,
		})
	case *ast.Merge:
		var ok bool
		if len(seq) > 0 {
			switch seq[len(seq)-1].(type) {
			case *dag.Fork, *dag.Switch:
				ok = true
			}
		}
		if !ok {
			a.error(o, errors.New("merge operator must follow fork or switch"))
		}
		var exprs []dag.SortExpr
		for _, e := range o.Exprs {
			exprs = append(exprs, a.semSortExpr(nil, e, false))
		}
		return append(seq, &dag.Merge{Kind: "Merge", Exprs: exprs})
	case *ast.Unnest:
		e := a.semExpr(o.Expr)
		a.enterScope()
		defer a.exitScope()
		var body dag.Seq
		if o.Body != nil {
			body = a.semSeq(o.Body)
		}
		return append(seq, &dag.Unnest{
			Kind: "Unnest",
			Expr: e,
			Body: body,
		})
	case *ast.Shapes:
		e := dag.Expr(&dag.This{Kind: "This"})
		if o.Expr != nil {
			e = a.semExpr(o.Expr)
		}
		seq = append(seq, &dag.Filter{
			Kind: "Filter",
			Expr: &dag.UnaryExpr{
				Kind:    "UnaryExpr",
				Op:      "!",
				Operand: &dag.IsNullExpr{Kind: "IsNullExpr", Expr: e},
			},
		})
		seq = append(seq, &dag.Aggregate{
			Kind: "Aggregate",
			Aggs: []dag.Assignment{
				{
					Kind: "Assignment",
					LHS:  pathOf("sample"),
					RHS:  &dag.Agg{Kind: "Agg", Name: "any", Expr: e},
				},
			},
			Keys: []dag.Assignment{
				{
					Kind: "Assignment",
					LHS:  pathOf("shape"),
					RHS:  &dag.Call{Kind: "Call", Name: "typeof", Args: []dag.Expr{e}},
				},
			},
		})
		return append(seq, dag.NewValues(&dag.This{Kind: "This", Path: field.Path{"sample"}}))
	case *ast.Assert:
		cond := a.semExpr(o.Expr)
		// 'assert EXPR' is equivalent to
		// 'values EXPR ? this : error({message: "assertion failed", "expr": EXPR_text, "on": this}'
		// where EXPR_text is the literal text of EXPR.
		return append(seq, dag.NewValues(
			&dag.Conditional{
				Kind: "Conditional",
				Cond: cond,
				Then: &dag.This{Kind: "This"},
				Else: &dag.Call{
					Kind: "Call",
					Name: "error",
					Args: []dag.Expr{&dag.RecordExpr{
						Kind: "RecordExpr",
						Elems: []dag.RecordElem{
							&dag.Field{
								Kind:  "Field",
								Name:  "message",
								Value: &dag.Literal{Kind: "Literal", Value: `"assertion failed"`},
							},
							&dag.Field{
								Kind:  "Field",
								Name:  "expr",
								Value: &dag.Literal{Kind: "Literal", Value: sup.QuotedString(o.Text)},
							},
							&dag.Field{
								Kind:  "Field",
								Name:  "on",
								Value: &dag.This{Kind: "This"},
							},
						},
					}},
				},
			}))
	case *ast.Values:
		exprs := a.semExprs(o.Exprs)
		return append(seq, dag.NewValues(exprs...))
	case *ast.Load:
		if !a.env.IsLake() {
			a.error(o, errors.New("load operator cannot be used without a lake"))
			return dag.Seq{badOp()}
		}
		poolID, err := lakeparse.ParseID(o.Pool.Text)
		if err != nil {
			poolID, err = a.env.PoolID(a.ctx, o.Pool.Text)
			if err != nil {
				a.error(o, err)
				return append(seq, badOp())
			}
		}
		opArgs := a.semOpArgs(o.Args, "commit", "author", "message", "meta")
		branch, _ := a.textArg(opArgs, "commit")
		author, _ := a.textArg(opArgs, "author")
		message, _ := a.textArg(opArgs, "message")
		meta, _ := a.textArg(opArgs, "meta")
		return append(seq, &dag.Load{
			Kind:    "Load",
			Pool:    poolID,
			Branch:  branch,
			Author:  author,
			Message: message,
			Meta:    meta,
		})
	case *ast.Output:
		out := &dag.Output{Kind: "Output", Name: o.Name.Name}
		a.outputs[out] = o
		return append(seq, out)
	}
	panic(fmt.Errorf("semantic transform: unknown AST operator type: %T", o))
}

func (a *analyzer) singletonAgg(agg ast.Assignment, seq dag.Seq) dag.Seq {
	if agg.LHS != nil {
		return nil
	}
	out := a.semAssignment(agg)
	this, ok := out.LHS.(*dag.This)
	if !ok || len(this.Path) != 1 {
		return nil
	}
	return append(seq,
		&dag.Aggregate{
			Kind: "Aggregate",
			Aggs: []dag.Assignment{out},
		},
		dag.NewValues(this),
	)
}

func (a *analyzer) semDecls(decls []ast.Decl) ([]dag.Def, []*dag.Func) {
	var consts []dag.Def
	var fnDecls []*ast.FuncDecl
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.ConstDecl:
			consts = append(consts, a.semConstDecl(d))
		case *ast.FuncDecl:
			fnDecls = append(fnDecls, d)
		case *ast.OpDecl:
			a.semOpDecl(d)
		case *ast.TypeDecl:
			consts = append(consts, a.semTypeDecl(d))
		default:
			panic(fmt.Errorf("invalid declaration type %T", d))
		}
	}
	funcs := a.semFuncDecls(fnDecls)
	return consts, funcs
}

func (a *analyzer) semConstDecl(c *ast.ConstDecl) dag.Def {
	e := a.semExpr(c.Expr)
	if err := a.scope.DefineConst(a.sctx, c.Name, e); err != nil {
		a.error(c, err)
	}
	return dag.Def{
		Name: c.Name.Name,
		Expr: e,
	}
}

func (a *analyzer) semTypeDecl(d *ast.TypeDecl) dag.Def {
	typ, err := a.semType(d.Type)
	if err != nil {
		a.error(d.Type, err)
		typ = "null"
	}
	e := &dag.Literal{
		Kind:  "Literal",
		Value: fmt.Sprintf("<%s=%s>", sup.QuotedName(d.Name.Name), typ),
	}
	if err := a.scope.DefineConst(a.sctx, d.Name, e); err != nil {
		a.error(d.Name, err)
	}
	return dag.Def{Name: d.Name.Name, Expr: e}
}

func (a *analyzer) semFuncDecls(decls []*ast.FuncDecl) []*dag.Func {
	funcs := make([]*dag.Func, 0, len(decls))
	for _, d := range decls {
		var params []string
		for _, p := range d.Params {
			params = append(params, p.Name)
		}
		f := &dag.Func{
			Kind:   "Func",
			Name:   d.Name.Name,
			Params: params,
		}
		if err := a.scope.DefineAs(d.Name, f); err != nil {
			a.error(d.Name, err)
		}
		funcs = append(funcs, f)
	}
	for i, d := range decls {
		a.enterScope()
		funcs[i].Expr = a.semExpr(d.Expr)
		a.exitScope()
	}
	return funcs
}

func (a *analyzer) semOpDecl(d *ast.OpDecl) {
	m := make(map[string]bool)
	for _, p := range d.Params {
		if m[p.Name] {
			a.error(p, fmt.Errorf("duplicate parameter %q", p.Name))
			a.scope.DefineAs(d.Name, &opDecl{bad: true})
			return
		}
		m[p.Name] = true
	}
	if err := a.scope.DefineAs(d.Name, &opDecl{ast: d, scope: a.scope}); err != nil {
		a.error(d, err)
	}
}

func (a *analyzer) semOpAssignment(p *ast.OpAssignment) dag.Op {
	var aggs, puts []dag.Assignment
	for _, astAssign := range p.Assignments {
		// Parition assignments into agg vs. puts.
		assign := a.semAssignment(astAssign)
		if _, ok := assign.RHS.(*dag.Agg); ok {
			if _, ok := assign.LHS.(*dag.This); !ok {
				a.error(astAssign.LHS, errors.New("aggregate output field must be static"))
			}
			aggs = append(aggs, assign)
		} else {
			puts = append(puts, assign)
		}
	}
	if len(puts) > 0 && len(aggs) > 0 {
		a.error(p, errors.New("mix of aggregations and non-aggregations in assignment list"))
		return badOp()
	}
	if len(puts) > 0 {
		return &dag.Put{
			Kind: "Put",
			Args: puts,
		}
	}
	return &dag.Aggregate{
		Kind: "Aggregate",
		Aggs: aggs,
	}
}

func (a *analyzer) checkStaticAssignment(asts []ast.Assignment, assignments []dag.Assignment) bool {
	for k, assign := range assignments {
		if _, ok := assign.LHS.(*dag.BadExpr); ok {
			continue
		}
		if _, ok := assign.LHS.(*dag.This); !ok {
			a.error(asts[k].LHS, errors.New("output field must be static"))
			return true
		}
	}
	return false
}

func (a *analyzer) semOpExpr(e ast.Expr, seq dag.Seq) dag.Seq {
	if call, ok := e.(*ast.Call); ok {
		if seq := a.semCallOp(call, seq); seq != nil {
			return seq
		}
	}
	out := a.semExpr(e)
	if a.isBool(out) {
		return append(seq, dag.NewFilter(out))
	}
	return append(seq, dag.NewValues(out))
}

func (a *analyzer) isBool(e dag.Expr) bool {
	switch e := e.(type) {
	case *dag.Literal:
		return e.Value == "true" || e.Value == "false"
	case *dag.UnaryExpr:
		return a.isBool(e.Operand)
	case *dag.BinaryExpr:
		switch e.Op {
		case "and", "or", "in", "==", "!=", "<", "<=", ">", ">=":
			return true
		default:
			return false
		}
	case *dag.Conditional:
		return a.isBool(e.Then) && a.isBool(e.Else)
	case *dag.Call:
		// If udf recurse to inner expression.
		if f, _ := a.scope.LookupExpr(e.Name); f != nil {
			return a.isBool(f.(*dag.Func).Expr)
		}
		if e.Name == "cast" {
			if len(e.Args) != 2 {
				return false
			}
			if typval, ok := e.Args[1].(*dag.Literal); ok {
				return typval.Value == "bool"
			}
			return false
		}
		return function.HasBoolResult(e.Name)
	case *dag.IsNullExpr:
		return true
	case *dag.Search, *dag.RegexpMatch, *dag.RegexpSearch:
		return true
	default:
		return false
	}
}

func (a *analyzer) semCallOp(call *ast.Call, seq dag.Seq) dag.Seq {
	if body := a.maybeConvertUserOp(call); body != nil {
		return append(seq, body...)
	}
	name := call.Name.Name
	if agg := a.maybeConvertAgg(call); agg != nil {
		aggregate := &dag.Aggregate{
			Kind: "Aggregate",
			Aggs: []dag.Assignment{
				{
					Kind: "Assignment",
					LHS:  pathOf(name),
					RHS:  agg,
				},
			},
		}
		values := dag.NewValues(&dag.This{Kind: "This", Path: field.Path{name}})
		return append(append(seq, aggregate), values)
	}
	if !function.HasBoolResult(strings.ToLower(name)) {
		return nil
	}
	c := a.semCall(call)
	return append(seq, dag.NewFilter(c))
}

// maybeConvertUserOp returns nil, nil if the call is determined to not be a
// UserOp, otherwise it returns the compiled op or the encountered error.
func (a *analyzer) maybeConvertUserOp(call *ast.Call) dag.Seq {
	decl, err := a.scope.lookupOp(call.Name.Name)
	if decl == nil {
		return nil
	}
	if err != nil {
		a.error(call, err)
		return dag.Seq{badOp()}
	}
	if decl.bad {
		return dag.Seq{badOp()}
	}
	if call.Where != nil {
		a.error(call, errors.New("user defined operators cannot have a where clause"))
		return dag.Seq{badOp()}
	}
	params, args := decl.ast.Params, call.Args
	if len(params) != len(args) {
		a.error(call, fmt.Errorf("%d arg%s provided when operator expects %d arg%s", len(params), plural.Slice(params, "s"), len(args), plural.Slice(args, "s")))
		return dag.Seq{badOp()}
	}
	exprs := make([]dag.Expr, len(decl.ast.Params))
	for i, arg := range args {
		e := a.semExpr(arg)
		// Transform non-path arguments into literals.
		if _, ok := e.(*dag.This); !ok {
			val, err := rungen.EvalAtCompileTime(a.sctx, e)
			if err != nil {
				a.error(arg, err)
				exprs[i] = badExpr()
				continue
			}
			if val.IsError() {
				if val.IsMissing() {
					a.error(arg, errors.New("non-path arguments cannot have variable dependency"))
				} else {
					a.error(arg, errors.New(string(val.Bytes())))
				}
			}
			e = &dag.Literal{
				Kind:  "Literal",
				Value: sup.FormatValue(val),
			}
		}
		exprs[i] = e
	}
	if slices.Contains(a.opStack, decl.ast) {
		a.error(call, opCycleError(append(a.opStack, decl.ast)))
		return dag.Seq{badOp()}
	}
	a.opStack = append(a.opStack, decl.ast)
	oldscope := a.scope
	a.scope = NewScope(decl.scope)
	defer func() {
		a.opStack = a.opStack[:len(a.opStack)-1]
		a.scope = oldscope
	}()
	for i, p := range params {
		if err := a.scope.DefineAs(p, exprs[i]); err != nil {
			a.error(call, err)
			return dag.Seq{badOp()}
		}
	}
	return a.semSeq(decl.ast.Body)
}

func (a *analyzer) semOpArgs(args []ast.OpArg, allowed ...string) opArgs {
	guard := make(map[string]struct{})
	for _, s := range allowed {
		guard[s] = struct{}{}
	}
	return a.semOpArgsAny(args, guard)
}

func (a *analyzer) semOpArgsAny(args []ast.OpArg, allowed map[string]struct{}) opArgs {
	opArgs := make(opArgs)
	for _, arg := range args {
		switch arg := arg.(type) {
		case *ast.ArgText:
			key := strings.ToLower(arg.Key)
			if _, ok := opArgs[key]; ok {
				a.error(arg, fmt.Errorf("duplicate argument %q", arg.Key))
				continue
			}
			if _, ok := allowed[key]; !ok {
				a.error(arg, fmt.Errorf("unknown argument %q", arg.Key))
				continue
			}
			opArgs[key] = arg
		case *ast.ArgExpr:
			key := strings.ToLower(arg.Key)
			if _, ok := opArgs[key]; ok {
				a.error(arg, fmt.Errorf("duplicate argument %q", arg.Key))
				continue
			}
			if _, ok := allowed[key]; !ok {
				a.error(arg, fmt.Errorf("unknown argument %q", arg.Key))
				continue
			}
			opArgs[key] = argExpr{arg: arg, expr: a.semExpr(arg.Value)}
		default:
			panic(fmt.Sprintf("unknown arg type %T", arg))
		}
	}
	return opArgs
}

type opArgs map[string]any

type argExpr struct {
	arg  *ast.ArgExpr
	expr dag.Expr
}

func (a *analyzer) textArg(o opArgs, key string) (string, ast.Loc) {
	if v, ok := o[key]; ok {
		if arg, ok := v.(*ast.ArgText); ok {
			return arg.Value.Text, arg.Loc
		}
		// The PEG parser currently doesn't allow this but this may change.
		a.error(v.(*ast.ArgExpr).Loc, fmt.Errorf("argument %q must be plain text", key))
	}
	return "", ast.Loc{}
}

func (a *analyzer) exprArg(o opArgs, key string) (dag.Expr, ast.Loc) {
	if v, ok := o[key]; ok {
		if arg, ok := v.(*argExpr); ok {
			return arg.expr, arg.arg.Loc
		}
		// The PEG parser currently doesn't allow this but this may change.
		a.error(v.(*ast.ArgText).Loc, fmt.Errorf("argument %q must be expression", key))
	}
	return nil, ast.Loc{}
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
