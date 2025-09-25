package semantic

import (
	"context"
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
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/plural"
	"github.com/brimdata/super/pkg/reglob"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/parquetio"
	"github.com/brimdata/super/sup"
	"github.com/segmentio/ksuid"
)

func (t *translator) semSeq(seq ast.Seq) sem.Seq {
	var converted sem.Seq
	for _, op := range seq {
		converted = t.semOp(op, converted)
	}
	return converted
}

func (t *translator) semFrom(from *ast.From, seq sem.Seq) (sem.Seq, schema) {
	if len(from.Elems) > 1 {
		t.error(from, errors.New("cross join implied by multiple elements in from clause is not yet supported"))
		return sem.Seq{badOp()}, badSchema()
	}
	return t.semFromElem(from.Elems[0], seq)
}

// semFromElem generates a DAG fragment to read from the various sources potentially
// with embedded SQL subexpressions and joins.  We return the schema of the
// the entity to support SQL scoping semantics.  The callee is responsible for
// wrapping the result in a record representing the schemafied data as the output
// here is simply the underlying data sequence.
func (t *translator) semFromElem(elem *ast.FromElem, seq sem.Seq) (sem.Seq, schema) {
	var sch schema
	seq, sch = t.semFromEntity(elem.Entity, elem.Alias, elem.Args, seq)
	if elem.Ordinality != nil {
		t.error(elem.Ordinality, errors.New("WITH ORDINALITY clause is not yet supported"))
		return sem.Seq{badOp()}, badSchema()
	}
	return seq, sch
}

func (t *translator) fromSchema(alias *ast.TableAlias, name string) schema {
	if alias != nil {
		name = alias.Name
		if len(alias.Columns) != 0 {
			t.error(alias, errors.New("cannot apply aliased columns to dynamically typed data"))
		}
	}
	return &dynamicSchema{name: name}
}

func (t *translator) semFromEntity(entity ast.FromEntity, alias *ast.TableAlias, args []ast.OpArg, seq sem.Seq) (sem.Seq, schema) {
	switch entity := entity.(type) {
	case *ast.Glob:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		s := t.fromSchema(alias, "")
		if t.env.IsAttached() {
			return t.semPoolFromRegexp(entity, reglob.Reglob(entity.Pattern), entity.Pattern, "glob", args), s
		}
		return sem.Seq{t.semFromFileGlob(entity, entity.Pattern, args)}, s
	case *ast.Regexp:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		if !t.env.IsAttached() {
			t.error(entity, errors.New("cannot use regular expression with from operator on local file system"))
			return seq, badSchema()
		}
		return t.semPoolFromRegexp(entity, entity.Pattern, entity.Pattern, "regexp", args), t.fromSchema(alias, "")
	case *ast.Text:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		if c, ok := t.scope.ctes[strings.ToLower(entity.Text)]; ok {
			return t.fromCTE(entity, c, alias)
		}
		if seq := t.scope.lookupQuery(entity.Text); seq != nil {
			return seq, &dynamicSchema{}
		}
		op, def := t.semFromName(entity, entity.Text, args)
		if op, ok := op.(*sem.FileScan); ok {
			if cols, ok := t.fileScanColumns(op); ok {
				schema := schema(&staticSchema{def, cols})
				seq, schema, err := derefSchemaWithAlias(op, schema, alias, sem.Seq{op})
				if err != nil {
					t.error(alias, err)
				}
				return seq, schema
			}
		}
		return sem.Seq{op}, t.fromSchema(alias, def)
	case *ast.ExprEntity:
		seq, def := t.semFromExpr(entity, args, seq)
		return seq, t.fromSchema(alias, def)
	case *ast.DBMeta:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, badSchema()
		}
		return sem.Seq{t.semDBMeta(entity)}, &dynamicSchema{}
	case *ast.SQLPipe:
		return t.semSQLPipe(entity, seq, alias)
	case *ast.SQLJoin:
		return t.semSQLJoin(entity, seq)
	case *ast.SQLCrossJoin:
		return t.semCrossJoin(entity, seq)
	default:
		panic(fmt.Sprintf("semFromEntity: unknown entity type: %T", entity))
	}
}

func (t *translator) fromCTE(entity ast.FromEntity, c *cte, alias *ast.TableAlias) (sem.Seq, schema) {
	if slices.Contains(t.cteStack, c) {
		t.error(entity, errors.New("recursive WITH relations not currently supported"))
		return sem.Seq{badOp()}, badSchema()
	}
	t.cteStack = append(t.cteStack, c)
	body, schema := t.semSQLPipe(c.ast.Body, nil, &ast.TableAlias{Name: c.ast.Name.Name})
	t.cteStack = t.cteStack[:len(t.cteStack)-1]
	seq, schema, err := derefSchemaWithAlias(entity, schema, alias, body)
	if err != nil {
		t.error(alias, err)
	}
	return seq, schema
}

// XXX this should find a type from the schema rather than the columns
func (t *translator) fileScanColumns(op *sem.FileScan) ([]string, bool) {
	if op.Format != "parquet" {
		return nil, false
	}
	uri, err := storage.ParseURI(op.Path)
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithCancel(t.ctx)
	defer cancel()
	sr, err := t.env.Engine().Get(ctx, uri)
	if err != nil {
		return nil, false
	}
	defer sr.Close()
	cols, err := parquetio.TopLevelFieldNames(sr)
	return cols, err == nil
}

func (t *translator) semFromExpr(entity *ast.ExprEntity, args []ast.OpArg, seq sem.Seq) (sem.Seq, string) {
	expr := t.semExpr(entity.Expr)
	val, ok := t.maybeEval(expr)
	if ok && !hasError(val) {
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, ""
		}
		return t.semFromConstVal(val, entity, args)
	}
	// This is an expression so set up a robot scanner that pulls values from
	// parent to decide what to scan.
	return append(seq, &sem.RobotScan{
		Node:   entity,
		Expr:   expr,
		Format: t.asFormatArg(args),
	}), ""
}

func hasError(val super.Value) bool {
	has := function.NewHasError()
	result := has.Call([]super.Value{val})
	return result.AsBool()
}

func (t *translator) hasFromParent(loc ast.Node, seq sem.Seq) sem.Seq {
	if len(seq) > 0 {
		t.error(loc, errors.New("from operator cannot have parent unless from argument is an expression"))
		return append(seq, badOp())
	}
	return nil
}

func (t *translator) semFromConstVal(val super.Value, entity *ast.ExprEntity, args []ast.OpArg) (sem.Seq, string) {
	if super.TypeUnder(val.Type()) == super.TypeString {
		op, name := t.semFromName(entity, val.AsString(), args)
		return sem.Seq{op}, name
	}
	vals, err := val.Elements()
	if err != nil {
		t.error(entity.Expr, fmt.Errorf("from expression requires a string but encountered %s", sup.String(val)))
		return sem.Seq{badOp()}, ""
	}
	names := make([]string, 0, len(vals))
	for _, val := range vals {
		if super.TypeUnder(val.Type()) != super.TypeString {
			t.error(entity.Expr, fmt.Errorf("from expression requires a string but encountered %s", sup.String(val)))
			return sem.Seq{badOp()}, ""
		}
		names = append(names, val.AsString())
	}
	if len(names) == 1 {
		op, _ := t.semFromName(entity, names[0], args)
		return sem.Seq{op}, names[0]
	}
	var paths []sem.Seq
	for _, name := range names {
		op, _ := t.semFromName(entity, name, args)
		paths = append(paths, sem.Seq{op})
	}
	return sem.Seq{
		&sem.ForkOp{
			Paths: paths,
		},
	}, ""
}

func (t *translator) semFromName(entity ast.FromEntity, name string, args []ast.OpArg) (sem.Op, string) {
	if isURL(name) {
		return t.semFromURL(entity, name, args), ""
	}
	prefix := strings.Split(filepath.Base(name), ".")[0]
	if t.env.IsAttached() {
		return t.semPool(entity, name, args), prefix
	}
	return t.semFile(name, args), prefix
}

func (t *translator) asFormatArg(args []ast.OpArg) string {
	opArgs := t.semOpArgs(args, "format")
	s, _ := t.textArg(opArgs, "format")
	return s
}

func (t *translator) semFile(name string, args []ast.OpArg) sem.Op {
	format := t.asFormatArg(args)
	if format == "" {
		format = sio.FormatFromPath(name)
	}
	return &sem.FileScan{
		Path:   name,
		Format: format,
	}
}

func (t *translator) semFromFileGlob(globLoc ast.Node, pattern string, args []ast.OpArg) sem.Op {
	names, err := filepath.Glob(pattern)
	if err != nil {
		t.error(globLoc, err)
		return badOp()
	}
	if len(names) == 0 {
		t.error(globLoc, errors.New("no file names match glob pattern"))
		return badOp()
	}
	if len(names) == 1 {
		return t.semFile(names[0], args)
	}
	paths := make([]sem.Seq, 0, len(names))
	for _, name := range names {
		paths = append(paths, sem.Seq{t.semFile(name, args)})
	}
	return &sem.ForkOp{
		Paths: paths,
	}
}

func (t *translator) semFromURL(urlLoc ast.Node, u string, args []ast.OpArg) sem.Op {
	_, err := url.ParseRequestURI(u)
	if err != nil {
		t.error(urlLoc, err)
		return badOp()
	}
	opArgs := t.semOpArgs(args, "format", "method", "body", "headers")
	format, _ := t.textArg(opArgs, "format")
	method, _ := t.textArg(opArgs, "method")
	body, _ := t.textArg(opArgs, "body")
	var headers map[string][]string
	if e, loc := t.exprArg(opArgs, "headers"); e != nil {
		if val, ok := t.mustEval(e); ok {
			headers, err = unmarshalHeaders(val)
			if err != nil {
				t.error(loc, err)
			}
		}
	}
	if format == "" {
		format = sio.FormatFromPath(u)
	}
	return &sem.HTTPScan{
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

func (t *translator) semPoolFromRegexp(entity ast.FromEntity, re, orig, which string, args []ast.OpArg) sem.Seq {
	poolNames, err := t.matchPools(re, orig, which)
	if err != nil {
		t.error(entity, err)
		return sem.Seq{badOp()}
	}
	var paths []sem.Seq
	for _, name := range poolNames {
		paths = append(paths, sem.Seq{t.semPool(entity, name, args)})
	}
	return sem.Seq{&sem.ForkOp{Paths: paths}}
}

func (t *translator) semSortExpr(sch schema, s ast.SortExpr, reverse bool) sem.SortExpr {
	var e sem.Expr
	if sch != nil {
		e = t.semExprSchema(sch, s.Expr)
		if which, ok := isOrdinal(t.sctx, e); ok {
			var err error
			if e, err = sch.resolveOrdinal(s, which); err != nil {
				t.error(s.Expr, err)
			}
		}
	} else {
		e = t.semExpr(s.Expr)
	}
	o := order.Asc
	if s.Order != nil {
		var err error
		if o, err = order.Parse(s.Order.Name); err != nil {
			t.error(s.Order, err)
		}
	}
	if reverse {
		o = !o
	}
	n := order.NullsLast
	if s.Nulls != nil {
		if err := n.UnmarshalText([]byte(s.Nulls.Name)); err != nil {
			t.error(s.Nulls, err)
		}
	}
	return sem.SortExpr{Node: s, Expr: e, Order: o, Nulls: n}
}

func (t *translator) semPool(entity ast.FromEntity, poolName string, args []ast.OpArg) sem.Op {
	opArgs := t.semOpArgs(args, "commit", "meta", "tap")
	poolID, err := t.env.PoolID(t.ctx, poolName)
	if err != nil {
		t.error(entity, err)
		return badOp()
	}
	var commitID ksuid.KSUID
	commit, commitLoc := t.textArg(opArgs, "commit")
	if commit != "" {
		if commitID, err = dbid.ParseID(commit); err != nil {
			commitID, err = t.env.CommitObject(t.ctx, poolID, commit)
			if err != nil {
				t.error(commitLoc, err)
				return badOp()
			}
		}
	}
	meta, metaLoc := t.textArg(opArgs, "meta")
	if meta != "" {
		if _, ok := dag.CommitMetas[meta]; ok {
			if commitID == ksuid.Nil {
				commitID, err = t.env.CommitObject(t.ctx, poolID, "main")
				if err != nil {
					t.error(metaLoc, err)
					return badOp()
				}
			}
			tapString, _ := t.textArg(opArgs, "tap")
			tap := tapString != ""
			return &sem.CommitMetaScan{
				Node:   entity,
				Meta:   meta,
				Pool:   poolID,
				Commit: commitID,
				Tap:    tap,
			}
		}
		if _, ok := dag.PoolMetas[meta]; ok {
			return &sem.PoolMetaScan{
				Node: entity,
				Meta: meta,
				ID:   poolID,
			}
		}
		t.error(metaLoc, fmt.Errorf("unknown metadata type %q", meta))
		return badOp()
	}
	if commitID == ksuid.Nil {
		// This trick here allows us to default to the main branch when
		// there is a "from pool" operator with no meta query or commit object.
		commitID, err = t.env.CommitObject(t.ctx, poolID, "main")
		if err != nil {
			t.error(entity, err)
			return badOp()
		}
	}
	return &sem.PoolScan{
		Node:   entity,
		ID:     poolID,
		Commit: commitID,
	}
}

func (t *translator) semDBMeta(entity *ast.DBMeta) sem.Op {
	meta := entity.Meta.Text
	if _, ok := dag.DBMetas[meta]; !ok {
		t.error(entity, fmt.Errorf("unknown database metadata type %q in from operator", meta))
		return badOp()
	}
	return &sem.DBMetaScan{
		Node: entity,
		Meta: meta,
	}
}

func (t *translator) semDelete(op *ast.Delete) sem.Op {
	if !t.env.IsAttached() {
		t.error(op, errors.New("deletion requires database"))
		return badOp()
	}
	poolID, err := t.env.PoolID(t.ctx, op.Pool)
	if err != nil {
		t.error(op, err)
		return badOp()
	}
	var commitID ksuid.KSUID
	if op.Branch != "" {
		var err error
		if commitID, err = dbid.ParseID(op.Branch); err != nil {
			commitID, err = t.env.CommitObject(t.ctx, poolID, op.Branch)
			if err != nil {
				t.error(op, err)
				return badOp()
			}
		}
	}
	return &sem.DeleteScan{
		Node:   op,
		ID:     poolID,
		Commit: commitID,
	}
}

func (t *translator) matchPools(pattern, origPattern, patternDesc string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	pools, err := t.env.DB().ListPools(t.ctx)
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

func (t *translator) semScope(op *ast.Scope) sem.Seq {
	t.scope = NewScope(t.scope)
	defer t.exitScope()
	t.semDecls(op.Decls)
	return t.semSeq(op.Body)
}

// semOp does a semantic analysis on a flowgraph to an
// intermediate representation that can be compiled into the runtime
// object.  Currently, it only replaces the aggregate duration with
// a bucket call on the ts and replaces FunctionCalls in op context
// with either an aggregate or filter op based on the function's name.
func (t *translator) semOp(o ast.Op, seq sem.Seq) sem.Seq {
	switch o := o.(type) {
	case *ast.SQLSelect, *ast.SQLLimitOffset, *ast.SQLOrderBy, *ast.SQLPipe, *ast.SQLUnion, *ast.SQLWith, *ast.SQLValues:
		seq, sch := t.semSQLOp(o, seq)
		seq, _ = derefSchema(o, sch, seq)
		return seq
	case *ast.From:
		seq, _ := t.semFrom(o, seq)
		return seq
	case *ast.DefaultScan:
		return append(seq, &sem.DefaultScan{})
	case *ast.Delete:
		if len(seq) > 0 {
			panic("analyzer.SemOp: delete scan cannot have parent in AST")
		}
		return sem.Seq{t.semDelete(o)}
	case *ast.Aggregate:
		keys := t.semAssignments(o.Keys)
		t.checkStaticAssignment(o.Keys, keys)
		if len(keys) == 0 && len(o.Aggs) == 1 {
			if seq := t.singletonAgg(&o.Aggs[0], seq); seq != nil {
				return seq
			}
		}
		aggs := t.semAssignments(o.Aggs)
		t.checkStaticAssignment(o.Aggs, aggs)
		// Note: InputSortDir is copied in here but it's not meaningful
		// coming from a parser AST, only from a worker using the DAG,
		// which is another reason why we need separate AST and sem.
		// Said another way, we don't want to do semantic analysis on a worker AST
		// as we presume that work had already been done and we just need
		// to execute it.  For now, the worker only uses a filter expression
		// so this code path isn't hit yet, but it uses this same entry point
		// and it will soon do other stuff so we need to put in place the
		// separation... see issue #2163.
		return append(seq, &sem.AggregateOp{
			Node:  o,
			Limit: o.Limit,
			Keys:  keys,
			Aggs:  aggs,
		})
	case *ast.Parallel:
		var paths []sem.Seq
		for _, seq := range o.Paths {
			paths = append(paths, t.semSeq(seq))
		}
		return append(seq, &sem.ForkOp{Paths: paths})
	case *ast.Scope:
		return append(seq, t.semScope(o)...)
	case *ast.Switch:
		var expr sem.Expr
		if o.Expr != nil {
			expr = t.semExpr(o.Expr)
		}
		var cases []sem.Case
		for _, c := range o.Cases {
			var e sem.Expr
			if c.Expr != nil {
				e = t.semExpr(c.Expr)
			} else if o.Expr == nil {
				// c.Expr == nil indicates the default case,
				// whose handling depends on p.Expr.
				e = &sem.LiteralExpr{
					Node:  o, //XXX?
					Value: "true",
				}
			}
			path := t.semSeq(c.Path)
			cases = append(cases, sem.Case{Expr: e, Path: path})
		}
		return append(seq, &sem.SwitchOp{
			Node:  o,
			Expr:  expr,
			Cases: cases,
		})
	case *ast.Cut:
		assignments := t.semAssignments(o.Args)
		// Collect static paths so we can check on what is available.
		var fields field.List
		for _, a := range assignments {
			if this, ok := a.LHS.(*sem.ThisExpr); ok {
				fields = append(fields, this.Path)
			}
		}
		if _, err := super.NewRecordBuilder(t.sctx, fields); err != nil {
			t.error(o.Args, err)
			return append(seq, badOp())
		}
		return append(seq, &sem.CutOp{
			Node: o,
			Args: assignments,
		})
	case *ast.Debug:
		e := t.semExprNullable(o.Expr)
		if e == nil {
			e = sem.NewThis(o.Expr, nil)
		}
		return append(seq, &sem.DebugOp{
			Node: o,
			Expr: e,
		})
	case *ast.Distinct:
		return append(seq, &sem.DistinctOp{
			Node: o,
			Expr: t.semExpr(o.Expr),
		})
	case *ast.Drop:
		args := t.semFields(o.Args)
		if len(args) == 0 {
			t.error(o, errors.New("no fields given"))
		}
		return append(seq, &sem.DropOp{
			Node: o,
			Args: args,
		})
	case *ast.Sort:
		var sortExprs []sem.SortExpr
		for _, e := range o.Exprs {
			sortExprs = append(sortExprs, t.semSortExpr(nil, e, o.Reverse))
		}
		return append(seq, &sem.SortOp{
			Node:    o,
			Exprs:   sortExprs,
			Reverse: o.Reverse && len(sortExprs) == 0,
		})
	case *ast.Head:
		count := 1
		if o.Count != nil {
			count = t.mustEvalPositiveInteger(o.Count)
		}
		return append(seq, &sem.HeadOp{
			Node:  o,
			Count: count,
		})
	case *ast.Tail:
		count := 1
		if o.Count != nil {
			count = t.mustEvalPositiveInteger(o.Count)
		}
		return append(seq, &sem.TailOp{
			Node:  o,
			Count: count,
		})
	case *ast.Skip:
		return append(seq, &sem.SkipOp{
			Node:  o,
			Count: t.mustEvalPositiveInteger(o.Count),
		})
	case *ast.Uniq:
		return append(seq, &sem.UniqOp{
			Node:  o,
			Cflag: o.Cflag,
		})
	case *ast.Pass:
		return append(seq, &sem.PassOp{Node: o})
	case *ast.OpExpr:
		return t.semOpExpr(o.Expr, seq)
	case *ast.CallOp:
		return t.semCallOp(o, seq)
	case *ast.Search:
		return append(seq, &sem.FilterOp{Node: o, Expr: t.semExpr(o.Expr)})
	case *ast.Where:
		return append(seq, &sem.FilterOp{Node: o, Expr: t.semExpr(o.Expr)})
	case *ast.Top:
		limit := 1
		if o.Limit != nil {
			l := t.semExpr(o.Limit)
			val, ok := t.mustEval(l) //XXX loc needs to be passed down, should be gettable from sem expr
			if !ok {
				return append(seq, badOp())
			}
			if !super.IsSigned(val.Type().ID()) {
				t.error(o.Limit, errors.New("limit argument must be an integer"))
				return append(seq, badOp())
			}
			if limit = int(val.Int()); limit < 1 {
				t.error(o.Limit, errors.New("limit argument value must be greater than 0"))
				return append(seq, badOp())
			}
		}
		var exprs []sem.SortExpr
		for _, e := range o.Exprs {
			exprs = append(exprs, t.semSortExpr(nil, e, o.Reverse))
		}
		return append(seq, &sem.TopOp{
			Node:    o,
			Limit:   limit,
			Exprs:   exprs,
			Reverse: o.Reverse && len(exprs) == 0,
		})
	case *ast.Put:
		assignments := t.semAssignments(o.Args)
		// We can do collision checking on static paths, so check what we can.
		var fields field.List
		for _, a := range assignments {
			if this, ok := a.LHS.(*sem.ThisExpr); ok {
				fields = append(fields, this.Path)
			}
		}
		if err := expr.CheckPutFields(fields); err != nil {
			t.error(o, err)
		}
		return append(seq, &sem.PutOp{
			Node: o,
			Args: assignments,
		})
	case *ast.OpAssignment:
		return append(seq, t.semOpAssignment(o))
	case *ast.Rename:
		var assignments []sem.Assignment
		for _, fa := range o.Args {
			assign := t.semAssignment(&fa)
			if !isLval(assign.RHS) {
				t.error(fa.RHS, fmt.Errorf("illegal right-hand side of assignment"))
			}
			// If both paths are static validate them. Otherwise this will be
			// done at runtime.
			lhs, lhsOk := assign.LHS.(*sem.ThisExpr)
			rhs, rhsOk := assign.RHS.(*sem.ThisExpr)
			if rhsOk && lhsOk {
				if err := expr.CheckRenameField(lhs.Path, rhs.Path); err != nil {
					t.error(&fa, err)
				}
			}
			assignments = append(assignments, assign)
		}
		return append(seq, &sem.RenameOp{
			Node: o,
			Args: assignments,
		})
	case *ast.Fuse:
		return append(seq, &sem.FuseOp{Node: o})
	case *ast.Join:
		leftAlias, rightAlias := "left", "right"
		if o.Alias != nil {
			leftAlias = o.Alias.Left.Name
			rightAlias = o.Alias.Right.Name
		}
		if leftAlias == rightAlias {
			t.error(o.Alias, errors.New("left and right join aliases cannot be the same"))
			return append(seq, badOp())
		}
		var cond sem.Expr
		if o.Cond != nil {
			cond = t.semJoinCond(o.Cond, leftAlias, rightAlias)
		}
		style := o.Style
		if style == "" {
			style = "inner"
		}
		join := &sem.JoinOp{
			Node:       o,
			Style:      style,
			LeftAlias:  leftAlias,
			RightAlias: rightAlias,
			Cond:       cond,
		}
		if o.RightInput == nil {
			return append(seq, join)
		}
		fork := &sem.ForkOp{
			Paths: []sem.Seq{
				seq,
				t.semSeq(o.RightInput),
			},
		}
		return sem.Seq{fork, join}
	case *ast.Explode:
		typ, err := t.semType(o.Type)
		if err != nil {
			t.error(o.Type, err)
			typ = "<bad type expr>"
		}
		args := t.semExprs(o.Args)
		var as string
		if o.As == nil {
			as = "value"
		} else {
			e := t.semExpr(o.As)
			this, ok := e.(*sem.ThisExpr)
			if !ok {
				t.error(o.As, errors.New("as clause must be a field reference"))
				return append(seq, badOp())
			} else if len(this.Path) != 1 {
				t.error(o.As, errors.New("field must be a top-level field"))
				return append(seq, badOp())
			}
			as = this.Path[0]
		}
		return append(seq, &sem.ExplodeOp{
			Node: o,
			Args: args,
			Type: typ,
			As:   as,
		})
	case *ast.Merge:
		var ok bool
		if len(seq) > 0 {
			switch seq[len(seq)-1].(type) {
			case *sem.ForkOp, *sem.SwitchOp:
				ok = true
			}
		}
		if !ok {
			t.error(o, errors.New("merge operator must follow fork or switch"))
		}
		var exprs []sem.SortExpr
		for _, e := range o.Exprs {
			exprs = append(exprs, t.semSortExpr(nil, e, false))
		}
		return append(seq, &sem.MergeOp{Node: o, Exprs: exprs})
	case *ast.Unnest:
		e := t.semExpr(o.Expr)
		t.enterScope()
		defer t.exitScope()
		var body sem.Seq
		if o.Body != nil {
			body = t.semSeq(o.Body)
		}
		return append(seq, &sem.UnnestOp{
			Node: o,
			Expr: e,
			Body: body,
		})
	case *ast.Shapes: // XXX move to std library?
		e := sem.Expr(sem.NewThis(o, nil))
		if o.Expr != nil {
			e = t.semExpr(o.Expr)
		}
		seq = append(seq, &sem.FilterOp{
			Node: o,
			Expr: sem.NewUnaryExpr(o, "!", &sem.IsNullExpr{Node: o, Expr: e}),
		})
		seq = append(seq, &sem.AggregateOp{
			Node: o,
			Aggs: []sem.Assignment{
				{
					Node: o,
					LHS:  sem.NewThis(o, []string{"sample"}),
					RHS:  &sem.AggFunc{Node: o, Name: "any", Expr: e},
				},
			},
			Keys: []sem.Assignment{
				{
					Node: o,
					LHS:  sem.NewThis(o, []string{"shape"}),
					RHS:  sem.NewCall(o, "typeof", []sem.Expr{e}),
				},
			},
		})
		return append(seq, sem.NewValues(o, sem.NewThis(o, []string{"sample"})))
	case *ast.Assert: // move this to stdlib (need expr fmt thunk to replace assert.Text)
		cond := t.semExpr(o.Expr)
		// 'assert EXPR' is equivalent to
		// 'values EXPR ? this : error({message: "assertion failed", "expr": EXPR_text, "on": this}'
		// where EXPR_text is the literal text of EXPR.
		return append(seq, sem.NewValues(o,
			&sem.CondExpr{
				Node: o,
				Cond: cond,
				Then: sem.NewThis(o, nil),
				Else: sem.NewCall(
					nil, //XXX
					"error",
					[]sem.Expr{&sem.RecordExpr{
						Node: o,
						Elems: []sem.RecordElem{
							&sem.FieldElem{
								Node:  o,
								Name:  "message",
								Value: &sem.LiteralExpr{Node: o, Value: `"assertion failed"`},
							},
							&sem.FieldElem{
								Node:  o,
								Name:  "expr",
								Value: &sem.LiteralExpr{Node: o, Value: sup.QuotedString(o.Text)},
							},
							&sem.FieldElem{
								Node:  o,
								Name:  "on",
								Value: sem.NewThis(nil /*XXX*/, nil),
							},
						},
					}},
				),
			}))
	case *ast.Values:
		return append(seq, sem.NewValues(o, t.semExprs(o.Exprs)...))
	case *ast.Load:
		if !t.env.IsAttached() {
			t.error(o, errors.New("load operator cannot be used without an attached database"))
			return sem.Seq{badOp()}
		}
		poolID, err := dbid.ParseID(o.Pool.Text)
		if err != nil {
			poolID, err = t.env.PoolID(t.ctx, o.Pool.Text)
			if err != nil {
				t.error(o, err)
				return append(seq, badOp())
			}
		}
		opArgs := t.semOpArgs(o.Args, "commit", "author", "message", "meta")
		branch, _ := t.textArg(opArgs, "commit")
		author, _ := t.textArg(opArgs, "author")
		message, _ := t.textArg(opArgs, "message")
		meta, _ := t.textArg(opArgs, "meta")
		return append(seq, &sem.LoadOp{
			Node:    o,
			Pool:    poolID,
			Branch:  branch,
			Author:  author,
			Message: message,
			Meta:    meta,
		})
	case *ast.Output:
		return append(seq, &sem.OutputOp{Node: o, Name: o.Name.Name})
	}
	panic(o)
}

func (t *translator) singletonAgg(assignment *ast.Assignment, seq sem.Seq) sem.Seq {
	if assignment.LHS != nil {
		return nil
	}
	out := t.semAssignment(assignment)
	this, ok := out.LHS.(*sem.ThisExpr)
	if !ok || len(this.Path) != 1 {
		return nil
	}
	return append(seq,
		&sem.AggregateOp{
			Node: assignment,
			Aggs: []sem.Assignment{out},
		},
		sem.NewValues(assignment, this),
	)
}

func (t *translator) semDecls(decls []ast.Decl) {
	var funcDefs []*sem.FuncDef
	var bodies []ast.Expr
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.ConstDecl:
			// XXX Consts don't quite work right.  We should bind them all to a DAG
			// placeholder and substitute them in as thunks, then do constant eval
			// at the very end of semantic analysis.  To be done in a later PR.
			t.semConstDecl(d)
		case *ast.FuncDecl:
			// We need to declare functions in the symbol table (so ops can find them)
			// but not try to process the function body (because they can refer to ops).
			// So we declare all the functions, process the ops, then process the function
			// bodies.
			funcDefs = append(funcDefs, t.declareFunc(d))
			bodies = append(bodies, d.Lambda.Expr)
		case *ast.OpDecl:
			t.semOpDecl(d)
		case *ast.QueryDecl:
			t.semQueryDecl(d)
		case *ast.TypeDecl:
			t.semTypeDecl(d)
		default:
			panic(fmt.Errorf("invalid declaration type %T", d))
		}
	}
	t.semFuncDefs(funcDefs, bodies)
}

func (t *translator) semConstDecl(c *ast.ConstDecl) {
	e := t.semExpr(c.Expr)
	if err := t.evalAndBindConst(c.Name.Name, e); err != nil {
		t.error(c, err)
	}
}

func (t *translator) semQueryDecl(d *ast.QueryDecl) {
	if err := t.scope.BindSymbol(d.Name.Name, t.semSeq(d.Body)); err != nil {
		t.error(d.Name, err)
	}
}

func (t *translator) semTypeDecl(d *ast.TypeDecl) {
	typ, err := t.semType(d.Type)
	if err != nil {
		t.error(d.Type, err)
		typ = "null"
	}
	e := &sem.LiteralExpr{
		Node:  d.Name,
		Value: fmt.Sprintf("<%s=%s>", sup.QuotedName(d.Name.Name), typ),
	}
	if err := t.evalAndBindConst(d.Name.Name, e); err != nil {
		t.error(d.Name, err)
	}
}

func idsAsStrings(ids []*ast.ID) []string {
	out := make([]string, 0, len(ids))
	for _, p := range ids {
		out = append(out, p.Name)
	}
	return out
}

func (t *translator) declareFunc(d *ast.FuncDecl) *sem.FuncDef {
	tag := t.newFunc(d.Lambda, d.Name.Name, idsAsStrings(d.Lambda.Params), nil)
	funcDef := t.funcsByTag[tag]
	if err := t.scope.BindSymbol(d.Name.Name, funcDef); err != nil {
		t.error(d.Name, err)
	}
	return funcDef
}

func (t *translator) semFuncDefs(funcDefs []*sem.FuncDef, bodies []ast.Expr) {
	for i, funcDef := range funcDefs {
		t.enterScope()
		for _, p := range funcDef.Params {
			t.scope.BindSymbol(p, param{})
		}
		funcDef.Body = t.semExpr(bodies[i])
		t.exitScope()
	}
}

func (t *translator) semOpDecl(d *ast.OpDecl) {
	m := make(map[string]bool)
	for _, formal := range d.Params {
		if m[formal.Name] {
			t.error(formal, fmt.Errorf("duplicate parameter %q", formal.Name))
			t.scope.BindSymbol(formal.Name, &opDecl{bad: true})
			return
		}
		m[formal.Name] = true
	}
	if err := t.scope.BindSymbol(d.Name.Name, &opDecl{ast: d, scope: t.scope}); err != nil {
		t.error(d, err)
	}
}

func (t *translator) semOpAssignment(p *ast.OpAssignment) sem.Op {
	var aggs, puts []sem.Assignment
	for _, astAssign := range p.Assignments {
		// Parition assignments into agg vs. puts.
		assign := t.semAssignment(&astAssign)
		if _, ok := assign.RHS.(*sem.AggFunc); ok {
			if _, ok := assign.LHS.(*sem.ThisExpr); !ok {
				t.error(astAssign.LHS, errors.New("aggregate output field must be static"))
			}
			aggs = append(aggs, assign)
		} else {
			puts = append(puts, assign)
		}
	}
	if len(puts) > 0 && len(aggs) > 0 {
		t.error(p, errors.New("mix of aggregations and non-aggregations in assignment list"))
		return badOp()
	}
	if len(puts) > 0 {
		return &sem.PutOp{
			Node: p,
			Args: puts,
		}
	}
	return &sem.AggregateOp{
		Node: p,
		Aggs: aggs,
	}
}

func (t *translator) checkStaticAssignment(asts []ast.Assignment, assignments []sem.Assignment) bool {
	for k, assign := range assignments {
		if _, ok := assign.LHS.(*sem.BadExpr); ok {
			continue
		}
		if _, ok := assign.LHS.(*sem.ThisExpr); !ok {
			t.error(asts[k].LHS, errors.New("output field must be static"))
			return true
		}
	}
	return false
}

func (t *translator) semOpExpr(e ast.Expr, seq sem.Seq) sem.Seq {
	if call, ok := e.(*ast.Call); ok {
		if seq := t.semCallOpExpr(call, seq); seq != nil {
			return seq
		}
	}
	out := t.semExpr(e)
	if t.isBool(out) {
		return append(seq, &sem.FilterOp{Node: e, Expr: out})
	}
	return append(seq, sem.NewValues(e, out))
}

func (t *translator) isBool(e sem.Expr) bool {
	switch e := e.(type) {
	case *sem.LiteralExpr:
		return e.Value == "true" || e.Value == "false"
	case *sem.UnaryExpr:
		return t.isBool(e.Operand)
	case *sem.BinaryExpr:
		switch e.Op {
		case "and", "or", "in", "==", "!=", "<", "<=", ">", ">=":
			return true
		default:
			return false
		}
	case *sem.CondExpr:
		return t.isBool(e.Then) && t.isBool(e.Else)
	case *sem.CallExpr:
		if f := t.funcsByTag[e.Tag]; f != nil {
			return t.isBool(f.Body)
		}
		if e.Tag == "cast" {
			if len(e.Args) != 2 {
				return false
			}
			if typval, ok := e.Args[1].(*sem.LiteralExpr); ok {
				return typval.Value == "bool"
			}
			return false
		}
		return function.HasBoolResult(e.Tag)
	case *sem.IsNullExpr:
		return true
	case *sem.SearchTermExpr, *sem.RegexpMatchExpr, *sem.RegexpSearchExpr:
		return true
	default:
		return false
	}
}

func (t *translator) semCallOpExpr(call *ast.Call, seq sem.Seq) sem.Seq {
	f, ok := call.Func.(*ast.FuncName)
	if !ok {
		return nil
	}
	name := f.Name
	if agg := t.maybeConvertAgg(call); agg != nil {
		aggregate := &sem.AggregateOp{
			Node: call,
			Aggs: []sem.Assignment{
				{
					Node: call,
					LHS:  sem.NewThis(f, []string{name}),
					RHS:  agg,
				},
			},
		}
		values := sem.NewValues(call, sem.NewThis(call, []string{name}))
		return append(append(seq, aggregate), values)
	}
	if !function.HasBoolResult(strings.ToLower(name)) {
		return nil
	}
	return append(seq, &sem.FilterOp{Node: call, Expr: t.semCall(call)})
}

func (t *translator) semCallOp(call *ast.CallOp, seq sem.Seq) sem.Seq {
	decl, err := t.scope.lookupOp(call.Name.Name)
	if err != nil {
		t.error(call, err)
		return sem.Seq{badOp()}
	}
	if decl == nil {
		t.error(call, fmt.Errorf("no such user operator: %q", call.Name.Name))
		return sem.Seq{badOp()}
	}
	if decl.bad {
		return sem.Seq{badOp()}
	}
	return append(seq, t.semUserOp(call.Loc, decl, call.Args)...)
}

func (t *translator) semUserOp(loc ast.Loc, decl *opDecl, args []ast.Expr) sem.Seq {
	// We've found a user op bound to the name being invoked, so we pull out the
	// AST elements that were stashed from the definition of the user op and subsitute
	// them into the call site here.  This is essentially a thunk... each use of the
	// user op is compiled into the context in which it appears and all the references
	// in that expression are bound appropriately with respect to this context.
	params := decl.ast.Params
	if len(params) != len(args) {
		t.error(loc, fmt.Errorf("%d arg%s provided when operator expects %d arg%s", len(args), plural.Slice(args, "s"), len(params), plural.Slice(params, "s")))
		return sem.Seq{badOp()}
	}
	exprs := make([]sem.Expr, 0, len(args))
	for _, arg := range args {
		exprs = append(exprs, t.semExpr(arg))
	}
	if slices.Contains(t.opStack, decl.ast) {
		t.error(loc, opCycleError(append(t.opStack, decl.ast)))
		return sem.Seq{badOp()}
	}
	t.opStack = append(t.opStack, decl.ast)
	oldscope := t.scope
	t.scope = NewScope(decl.scope)
	defer func() {
		t.opStack = t.opStack[:len(t.opStack)-1]
		t.scope = oldscope
	}()
	for i, param := range params {
		if err := t.scope.BindSymbol(param.Name, exprs[i]); err != nil {
			t.error(loc, err)
			return sem.Seq{badOp()}
		}
	}
	return t.semSeq(decl.ast.Body)
}

func (t *translator) semOpArgs(args []ast.OpArg, allowed ...string) opArgs {
	guard := make(map[string]struct{})
	for _, s := range allowed {
		guard[s] = struct{}{}
	}
	return t.semOpArgsAny(args, guard)
}

func (t *translator) semOpArgsAny(args []ast.OpArg, allowed map[string]struct{}) opArgs {
	opArgs := make(opArgs)
	for _, arg := range args {
		switch arg := arg.(type) {
		case *ast.ArgText:
			key := strings.ToLower(arg.Key)
			if _, ok := opArgs[key]; ok {
				t.error(arg, fmt.Errorf("duplicate argument %q", arg.Key))
				continue
			}
			if _, ok := allowed[key]; !ok {
				t.error(arg, fmt.Errorf("unknown argument %q", arg.Key))
				continue
			}
			opArgs[key] = arg
		case *ast.ArgExpr:
			key := strings.ToLower(arg.Key)
			if _, ok := opArgs[key]; ok {
				t.error(arg, fmt.Errorf("duplicate argument %q", arg.Key))
				continue
			}
			if _, ok := allowed[key]; !ok {
				t.error(arg, fmt.Errorf("unknown argument %q", arg.Key))
				continue
			}
			opArgs[key] = argExpr{arg: arg, expr: t.semExpr(arg.Value)}
		default:
			panic(fmt.Sprintf("unknown arg type %T", arg))
		}
	}
	return opArgs
}

type opArgs map[string]any

type argExpr struct {
	arg  *ast.ArgExpr
	expr sem.Expr
}

func (t *translator) textArg(o opArgs, key string) (string, ast.Loc) {
	if v, ok := o[key]; ok {
		if arg, ok := v.(*ast.ArgText); ok {
			return arg.Value.Text, arg.Loc
		}
		// The PEG parser currently doesn't allow this but this may change.
		t.error(v.(*ast.ArgExpr).Loc, fmt.Errorf("argument %q must be plain text", key))
	}
	return "", ast.Loc{}
}

func (t *translator) exprArg(o opArgs, key string) (sem.Expr, ast.Loc) {
	if v, ok := o[key]; ok {
		if arg, ok := v.(*argExpr); ok {
			return arg.expr, arg.arg.Loc
		}
		// The PEG parser currently doesn't allow this but this may change.
		t.error(v.(*ast.ArgText).Loc, fmt.Errorf("argument %q must be expression", key))
	}
	return nil, ast.Loc{}
}

func (t *translator) mustEvalString(e sem.Expr) (field string, ok bool) {
	val, ok := t.mustEval(e)
	if ok && !val.IsError() && super.TypeUnder(val.Type()) == super.TypeString {
		return string(val.Bytes()), true
	}
	return "", false
}

func (t *translator) maybeEvalString(e sem.Expr) (field string, ok bool) {
	val, ok := t.maybeEval(e)
	if ok && !val.IsError() && super.TypeUnder(val.Type()) == super.TypeString {
		return string(val.Bytes()), true
	}
	return "", false
}

func (t *translator) mustEvalPositiveInteger(ae ast.Expr) int {
	e := t.semExpr(ae)
	val, ok := t.mustEval(e)
	if !ok {
		return 0
	}
	if !super.IsInteger(val.Type().ID()) || val.IsNull() {
		t.error(ae, fmt.Errorf("expression value must be an integer value: %s", sup.FormatValue(val)))
		return -1
	}
	v := int(val.AsInt())
	if v < 0 {
		t.error(ae, errors.New("expression value must be a positive integer"))
	}
	return v
}

func (t *translator) evalAndBindConst(name string, e sem.Expr) error {
	val, ok := t.mustEval(e)
	if !ok {
		return nil
	}
	literal := &sem.LiteralExpr{
		Node:  e,
		Value: sup.FormatValue(val),
	}
	return t.scope.BindSymbol(name, literal)
}

// mustEval leaves errors on the reporter and returns a bool as to whether
// the eval was successful
func (t *translator) mustEval(e sem.Expr) (super.Value, bool) {
	// We're in the middle of a semantic analysis but want to compile the
	// translated expression.  Operator thunks have been unfolded but
	// funcs haven't been resolved, but that's because we'll copy all the state
	// we need into a new instance of a translator (using the evaulator)
	// and we'll compile this all the way to a DAG an rungen it.  This is pretty
	// general because we need to handle things like subqueries that call
	// operator sequences that result in a constant value.
	return newEvaluator(t.reporter, t.funcsByTag).mustEval(t.sctx, e)
}

// maybeEVal leaves no errors behind and simply returns a value and bool
// indicating if the eval was successful
func (t *translator) maybeEval(e sem.Expr) (super.Value, bool) {
	return newEvaluator(t.reporter, t.funcsByTag).maybeEval(t.sctx, e)
}
