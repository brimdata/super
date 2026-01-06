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
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/sup"
	"github.com/segmentio/ksuid"
)

func (t *translator) seq(seq ast.Seq) sem.Seq {
	var converted sem.Seq
	for _, op := range seq {
		converted = t.semOp(op, converted)
	}
	return converted
}

func (t *translator) fromSource(entity ast.FromSource, args []ast.OpArg, seq sem.Seq) (sem.Seq, super.Type, string) {
	switch entity := entity.(type) {
	case *ast.GlobExpr:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, nil, ""
		}
		if t.env.IsAttached() {
			// XXX need to get fused type from pool
			return t.fromPoolRegexp(entity, reglob.Reglob(entity.Pattern), entity.Pattern, "glob", args), nil, ""
		}
		// XXX should fuse the types across the glob
		return sem.Seq{t.fromFileGlob(entity, entity.Pattern, args)}, nil, ""
	case *ast.RegexpExpr:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, nil, ""
		}
		if !t.env.IsAttached() {
			t.error(entity, errors.New("cannot use regular expression with from operator on local file system"))
			return seq, nil, ""
		}
		// XXX need to get fused type from pool
		return t.fromPoolRegexp(entity, entity.Pattern, entity.Pattern, "regexp", args), nil, ""
	case *ast.Text:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, nil, ""
		}
		if seq := t.scope.lookupQuery(t, entity.Text); seq != nil {
			// XXX call type checker on to compute type of query?
			return seq, nil, entity.Text
		}
		op, def := t.fromName(entity, entity.Text, args)
		if op, ok := op.(*sem.FileScan); ok {
			return sem.Seq{op}, op.Type, def
		}
		return sem.Seq{op}, nil, def
	case *ast.FromEval:
		seq, def := t.fromFString(entity, args, seq)
		return seq, nil, def
	case *ast.DBMeta:
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, nil, ""
		}
		return sem.Seq{t.dbMeta(entity)}, nil, ""
	default:
		panic(entity)
	}
}

func (t *translator) sqlTableExpr(e ast.SQLTableExpr, seq sem.Seq) (sem.Seq, schema) {
	switch e := e.(type) {
	case *ast.SQLFromItem:
		if e.Ordinality != nil {
			t.error(e.Ordinality, errors.New("WITH ORDINALITY clause is not yet supported"))
			return seq, badSchema
		}
		alias := e.Alias
		switch input := e.Input.(type) {
		case *ast.FromItem:
			var sch schema
			if c, name := t.maybeCTE(input.Source); c != nil {
				if bad := t.hasFromParent(input, seq); bad != nil {
					return bad, badSchema
				}
				if len(input.Args) != 0 {
					t.error(input, fmt.Errorf("CTE cannot use operator arguments"))
					return seq, badSchema
				}
				if alias == nil {
					alias = &ast.TableAlias{Name: name, Loc: c.Name.Loc}
				}
				seq, sch = t.fromCTE(input, c)
			} else {
				if _, ok := input.Source.(*ast.FromEval); !ok {
					if bad := t.hasFromParent(e, seq); bad != nil {
						return bad, badSchema
					}
				}
				var typ super.Type
				seq, typ, name = t.fromSource(input.Source, input.Args, seq)
				sch = newSchemaFromType(typ)
				if _, ok := sch.(*dynamicSchema); !ok && alias == nil {
					alias = &ast.TableAlias{Name: name, Loc: input.Loc}
				}
			}
			seq, sch, err := applyAlias(alias, sch, seq)
			if err != nil {
				t.error(alias, err)
			}
			return seq, sch
		case *ast.SQLPipe:
			seq, sch := t.sqlPipe(input, seq)
			seq, sch = sch.endScope(input, seq)
			seq, sch, err := applyAlias(alias, sch, seq)
			if err != nil {
				t.error(alias, err)
			}
			return seq, sch
		default:
			panic(input)
		}
	case *ast.SQLJoin:
		return t.sqlJoin(e, seq)
	case *ast.SQLCrossJoin:
		return t.sqlCrossJoin(e, seq)
	default:
		panic(e)
	}
}

func (t *translator) maybeCTE(source ast.FromSource) (*ast.SQLCTE, string) {
	if text, ok := source.(*ast.Text); ok {
		if c, ok := t.scope.ctes[strings.ToLower(text.Text)]; ok {
			return c, text.Text
		}
	}
	return nil, ""
}

func (t *translator) fromCTE(node ast.Node, c *ast.SQLCTE) (sem.Seq, schema) {
	if slices.Contains(t.cteStack, c) {
		t.error(node, errors.New("recursive WITH relations not currently supported"))
		return sem.Seq{badOp}, badSchema
	}
	t.cteStack = append(t.cteStack, c)
	seq, sch := t.sqlQueryBody(c.Body, nil, nil)
	// Add the CTE name as the alias.  If there is an actual alias, it will
	// override this.
	seq, sch = sch.endScope(node, seq)
	sch = addTableAlias(sch, c.Name.Name)
	t.cteStack = t.cteStack[:len(t.cteStack)-1]
	return sch.endScope(node, seq)

}

func (t *translator) fromFString(entity *ast.FromEval, args []ast.OpArg, seq sem.Seq) (sem.Seq, string) {
	expr := t.fstringExpr(entity.Expr)
	val, ok := t.maybeEval(expr)
	if ok && !hasError(val) {
		if bad := t.hasFromParent(entity, seq); bad != nil {
			return bad, ""
		}
		return t.fromConst(val, entity, args)
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
		return append(seq, badOp)
	}
	return nil
}

func (t *translator) fromConst(val super.Value, entity *ast.FromEval, args []ast.OpArg) (sem.Seq, string) {
	if super.TypeUnder(val.Type()) == super.TypeString {
		op, name := t.fromName(entity, val.AsString(), args)
		return sem.Seq{op}, name
	}
	vals, err := val.Elements()
	if err != nil {
		t.error(entity.Expr, fmt.Errorf("from expression requires a string but encountered %s", sup.String(val)))
		return sem.Seq{badOp}, ""
	}
	names := make([]string, 0, len(vals))
	for _, val := range vals {
		if super.TypeUnder(val.Type()) != super.TypeString {
			t.error(entity.Expr, fmt.Errorf("from expression requires a string but encountered %s", sup.String(val)))
			return sem.Seq{badOp}, ""
		}
		names = append(names, val.AsString())
	}
	if len(names) == 1 {
		op, _ := t.fromName(entity, names[0], args)
		return sem.Seq{op}, names[0]
	}
	var paths []sem.Seq
	for _, name := range names {
		op, _ := t.fromName(entity, name, args)
		paths = append(paths, sem.Seq{op})
	}
	return sem.Seq{
		&sem.ForkOp{
			Paths: paths,
		},
	}, ""
}

func (t *translator) fromName(node ast.Node, name string, args []ast.OpArg) (sem.Op, string) {
	if isURL(name) {
		return t.fromURL(node, name, args), ""
	}
	prefix := strings.Split(filepath.Base(name), ".")[0]
	if t.env.IsAttached() {
		return t.pool(node, name, args), prefix
	}
	return t.file(node, name, args), prefix
}

func (t *translator) asFormatArg(args []ast.OpArg) string {
	opArgs := t.opArgs(args, "format")
	s, _ := t.textArg(opArgs, "format")
	return s
}

func (t *translator) file(n ast.Node, name string, args []ast.OpArg) sem.Op {
	format := t.asFormatArg(args)
	if format == "" {
		format = sio.FormatFromPath(name)
	}
	typ, err := t.fileType(name, format)
	if err != nil {
		t.error(n, err)
		typ = badType
	}
	return &sem.FileScan{
		Node:   n,
		Type:   typ,
		Paths:  []string{name},
		Format: format,
	}
}

func (t *translator) fileType(path, format string) (super.Type, error) {
	if t.env.Dynamic {
		return nil, nil
	}
	engine := t.env.Engine()
	if engine == nil {
		return nil, nil
	}
	uri, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	r, err := engine.Get(t.ctx, uri)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var b [1]byte
	if _, err := r.ReadAt(b[:], 0); err != nil {
		// r can't seek so it's a fifo or pipe.
		return nil, nil
	}
	f, err := anyio.NewFile(t.sctx, r, path, anyio.ReaderOpts{Format: format})
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if typer, ok := f.Reader.(interface{ Type() super.Type }); ok {
		return typer.Type(), nil
	}
	fuser := t.checker.newFuser()
	for {
		val, err := f.Read()
		if val == nil || err != nil {
			return fuser.Type(), err
		}
		fuser.fuse(val.Type())
	}
}

func (t *translator) fromFileGlob(globLoc ast.Node, pattern string, args []ast.OpArg) sem.Op {
	names, err := filepath.Glob(pattern)
	if err != nil {
		t.error(globLoc, err)
		return badOp
	}
	if len(names) == 0 {
		t.error(globLoc, errors.New("no file names match glob pattern"))
		return badOp
	}
	if len(names) == 1 {
		return t.file(globLoc, names[0], args)
	}
	paths := make([]sem.Seq, 0, len(names))
	for _, name := range names {
		paths = append(paths, sem.Seq{t.file(globLoc, name, args)})
	}
	return &sem.ForkOp{
		Paths: paths,
	}
}

func (t *translator) fromURL(urlLoc ast.Node, u string, args []ast.OpArg) sem.Op {
	_, err := url.ParseRequestURI(u)
	if err != nil {
		t.error(urlLoc, err)
		return badOp
	}
	opArgs := t.opArgs(args, "format", "method", "body", "headers")
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
		Node:    urlLoc,
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

func (t *translator) fromPoolRegexp(node ast.Node, re, orig, which string, args []ast.OpArg) sem.Seq {
	poolNames, err := t.matchPools(re, orig, which)
	if err != nil {
		t.error(node, err)
		return sem.Seq{badOp}
	}
	var paths []sem.Seq
	for _, name := range poolNames {
		paths = append(paths, sem.Seq{t.pool(node, name, args)})
	}
	return sem.Seq{&sem.ForkOp{Paths: paths}}
}

func (t *translator) sortExpr(sch schema, s ast.SortExpr, reverse bool) sem.SortExpr {
	var e sem.Expr
	if sch != nil {
		if colno, ok := isOrdinal(s.Expr); ok {
			e = t.resolveOrdinalOuter(sch, s.Expr, "", colno)
		} else if selSch, ok := sch.(*selectSchema); ok {
			e = t.groupedExpr(selSch, s.Expr)
		} else {
			e = t.expr(s.Expr)
		}
	} else {
		e = t.expr(s.Expr)
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

func (t *translator) pool(node ast.Node, poolName string, args []ast.OpArg) sem.Op {
	opArgs := t.opArgs(args, "commit", "meta", "tap")
	poolID, err := t.env.PoolID(t.ctx, poolName)
	if err != nil {
		t.error(node, err)
		return badOp
	}
	var commitID ksuid.KSUID
	commit, commitLoc := t.textArg(opArgs, "commit")
	if commit != "" {
		if commitID, err = dbid.ParseID(commit); err != nil {
			commitID, err = t.env.CommitObject(t.ctx, poolID, commit)
			if err != nil {
				t.error(commitLoc, err)
				return badOp
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
					return badOp
				}
			}
			tapString, _ := t.textArg(opArgs, "tap")
			tap := tapString != ""
			return &sem.CommitMetaScan{
				Node:   node,
				Meta:   meta,
				Pool:   poolID,
				Commit: commitID,
				Tap:    tap,
			}
		}
		if _, ok := dag.PoolMetas[meta]; ok {
			return &sem.PoolMetaScan{
				Node: node,
				Meta: meta,
				ID:   poolID,
			}
		}
		t.error(metaLoc, fmt.Errorf("unknown metadata type %q", meta))
		return badOp
	}
	if commitID == ksuid.Nil {
		// This trick here allows us to default to the main branch when
		// there is a "from pool" operator with no meta query or commit object.
		commitID, err = t.env.CommitObject(t.ctx, poolID, "main")
		if err != nil {
			t.error(node, err)
			return badOp
		}
	}
	return &sem.PoolScan{
		Node:   node,
		ID:     poolID,
		Commit: commitID,
	}
}

func (t *translator) dbMeta(entity *ast.DBMeta) sem.Op {
	meta := entity.Meta.Text
	if _, ok := dag.DBMetas[meta]; !ok {
		t.error(entity, fmt.Errorf("unknown database metadata type %q in from operator", meta))
		return badOp
	}
	return &sem.DBMetaScan{
		Node: entity,
		Meta: meta,
	}
}

func (t *translator) deleteScan(op *ast.Delete) sem.Op {
	if !t.env.IsAttached() {
		t.error(op, errors.New("deletion requires database"))
		return badOp
	}
	poolID, err := t.env.PoolID(t.ctx, op.Pool)
	if err != nil {
		t.error(op, err)
		return badOp
	}
	var commitID ksuid.KSUID
	if op.Branch != "" {
		var err error
		if commitID, err = dbid.ParseID(op.Branch); err != nil {
			commitID, err = t.env.CommitObject(t.ctx, poolID, op.Branch)
			if err != nil {
				t.error(op, err)
				return badOp
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

func (t *translator) scopeOp(op *ast.ScopeOp) sem.Seq {
	t.scope = NewScope(t.scope)
	defer t.exitScope()
	t.decls(op.Decls)
	return t.seq(op.Body)
}

// semOp does a semantic analysis on a flowgraph to an
// intermediate representation that can be compiled into the runtime
// object.  Currently, it only replaces the aggregate duration with
// a bucket call on the ts and replaces FunctionCalls in op context
// with either an aggregate or filter op based on the function's name.
func (t *translator) semOp(o ast.Op, seq sem.Seq) sem.Seq {
	switch o := o.(type) {
	case *ast.SQLOp:
		seq, sch := t.sqlQueryBody(o.Body, nil, seq)
		return unfurl(o, sch, seq)
	case *ast.FileScan:
		format := t.env.ReaderOpts.Format
		fuser := t.checker.newFuser()
		paths := slices.Clone(o.Paths)
		for i, p := range paths {
			if p == "-" {
				p = "stdio:stdin"
				paths[i] = p
			}
			typ, err := t.fileType(p, format)
			if typ == nil || err != nil {
				if err != nil {
					t.reporter.AddError(p+": "+err.Error(), -1, -1)
				}
				fuser = nil
				break
			}
			fuser.fuse(typ)
		}
		var typ super.Type
		if fuser != nil {
			typ = fuser.Type()
		}
		return append(seq, &sem.FileScan{
			Node:   o,
			Type:   typ,
			Paths:  paths,
			Format: format,
		})
	case *ast.FromOp:
		seq, _, _ = t.fromSource(o.Item.Source, o.Item.Args, seq)
		return seq
	case *ast.DefaultScan:
		return append(seq, &sem.DefaultScan{Node: o})
	case *ast.Delete:
		if len(seq) > 0 {
			panic("analyzer.SemOp: delete scan cannot have parent in AST")
		}
		return sem.Seq{t.deleteScan(o)}
	case *ast.AggregateOp:
		keys := t.assignments(o.Keys)
		t.checkStaticAssignment(o.Keys, keys)
		if len(keys) == 0 && len(o.Aggs) == 1 {
			if seq := t.singletonAgg(&o.Aggs[0], seq); seq != nil {
				return seq
			}
		}
		if len(keys) == 1 && len(o.Aggs) == 0 {
			if seq := t.singletonKey(o.Keys[0], seq); seq != nil {
				return seq
			}
		}
		aggs := t.assignments(o.Aggs)
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
	case *ast.ForkOp:
		var paths []sem.Seq
		for _, seq := range o.Paths {
			paths = append(paths, t.seq(seq))
		}
		return append(seq, &sem.ForkOp{Paths: paths})
	case *ast.ScopeOp:
		return append(seq, t.scopeOp(o)...)
	case *ast.SwitchOp:
		var expr sem.Expr
		if o.Expr != nil {
			expr = t.expr(o.Expr)
		}
		var cases []sem.Case
		for _, c := range o.Cases {
			var e sem.Expr
			if c.Expr != nil {
				e = t.expr(c.Expr)
			} else if o.Expr == nil {
				// c.Expr == nil indicates the default case,
				// whose handling depends on p.Expr.
				e = &sem.LiteralExpr{
					Node:  o,
					Value: "true",
				}
			}
			path := t.seq(c.Path)
			cases = append(cases, sem.Case{Expr: e, Path: path})
		}
		return append(seq, &sem.SwitchOp{
			Node:  o,
			Expr:  expr,
			Cases: cases,
		})
	case *ast.CountOp:
		var alias string
		var expr sem.Expr
		if o.Expr == nil {
			alias = "count"
			expr = &sem.RecordExpr{
				Elems: []sem.RecordElem{
					&sem.FieldElem{Name: "that", Value: sem.NewThis(nil, nil)},
				},
			}
		} else {
			n := len(o.Expr.Elems)
			if n == 0 {
				t.error(o.Expr, errors.New("count record expression must not be empty"))
				return append(seq, badOp)
			}
			last := o.Expr.Elems[n-1]
			if exprElem, ok := last.(*ast.ExprElem); ok {
				if id, ok := exprElem.Expr.(*ast.IDExpr); ok {
					alias = id.Name
				}
			}
			if alias == "" {
				t.error(last, errors.New("last element in record expression for count must be an identifier"))
				return append(seq, badOp)
			}
			if len(o.Expr.Elems) > 1 {
				expr = t.expr(&ast.RecordExpr{
					Kind:  "RecordExpr",
					Elems: o.Expr.Elems[:n-1],
					Loc:   o.Expr.Loc,
				})
			}
		}
		return append(seq, &sem.CountOp{
			Node:  o,
			Alias: alias,
			Expr:  expr,
		})
	case *ast.CutOp:
		//XXX When cutting an lval with no LHS, promote the lval to the LHS so
		// it is not auto-inferred.  We will change cut to use paths in a future PR.
		// Currently there is work in optimizer and parallelizer to manage changing
		// the tests that use cut to use values instead.  This work needed to be done
		// anyway, but we don't want to change cut until we're ready to do that work.
		for k, arg := range o.Args {
			if arg.LHS == nil {
				rhs := t.expr(arg.RHS)
				if isLval(rhs) {
					o.Args[k].LHS = arg.RHS
				}
			}
		}
		assignments := t.assignments(o.Args)
		// Collect static paths so we can check on what is available.
		var fields field.List
		for _, a := range assignments {
			if this, ok := a.LHS.(*sem.ThisExpr); ok {
				fields = append(fields, this.Path)
			}
		}
		if _, err := super.NewRecordBuilder(t.sctx, fields); err != nil {
			t.error(o.Args, err)
			return append(seq, badOp)
		}
		return append(seq, &sem.CutOp{
			Node: o,
			Args: assignments,
		})
	case *ast.DebugOp:
		e := t.exprNullable(o.Expr)
		if e == nil {
			e = sem.NewThis(o.Expr, nil)
		}
		return append(seq, &sem.DebugOp{
			Node: o,
			Expr: e,
		})
	case *ast.DistinctOp:
		return append(seq, &sem.DistinctOp{
			Node: o,
			Expr: t.expr(o.Expr),
		})
	case *ast.DropOp:
		args := t.fields(o.Args)
		if len(args) == 0 {
			t.error(o, errors.New("no fields given"))
		}
		return append(seq, &sem.DropOp{
			Node: o,
			Args: args,
		})
	case *ast.SortOp:
		var sortExprs []sem.SortExpr
		for _, e := range o.Exprs {
			sortExprs = append(sortExprs, t.sortExpr(nil, e, o.Reverse))
		}
		return append(seq, &sem.SortOp{
			Node:    o,
			Exprs:   sortExprs,
			Reverse: o.Reverse && len(sortExprs) == 0,
		})
	case *ast.HeadOp:
		count := 1
		if o.Count != nil {
			count = t.mustEvalPositiveInteger(o.Count)
		}
		return append(seq, &sem.HeadOp{
			Node:  o,
			Count: count,
		})
	case *ast.TailOp:
		count := 1
		if o.Count != nil {
			count = t.mustEvalPositiveInteger(o.Count)
		}
		return append(seq, &sem.TailOp{
			Node:  o,
			Count: count,
		})
	case *ast.SkipOp:
		return append(seq, &sem.SkipOp{
			Node:  o,
			Count: t.mustEvalPositiveInteger(o.Count),
		})
	case *ast.UniqOp:
		return append(seq, &sem.UniqOp{
			Node:  o,
			Cflag: o.Cflag,
		})
	case *ast.PassOp:
		return append(seq, &sem.PassOp{Node: o})
	case *ast.ExprOp:
		return t.exprOp(o.Expr, seq)
	case *ast.CallOp:
		return t.callOp(o, seq)
	case *ast.SearchOp:
		return append(seq, &sem.FilterOp{Node: o, Expr: t.expr(o.Expr)})
	case *ast.WhereOp:
		return append(seq, &sem.FilterOp{Node: o, Expr: t.expr(o.Expr)})
	case *ast.TopOp:
		limit := 1
		if o.Limit != nil {
			l := t.expr(o.Limit)
			val, ok := t.mustEval(l)
			if !ok {
				return append(seq, badOp)
			}
			if !super.IsSigned(val.Type().ID()) {
				t.error(o.Limit, errors.New("limit argument must be an integer"))
				return append(seq, badOp)
			}
			if limit = int(val.Int()); limit < 1 {
				t.error(o.Limit, errors.New("limit argument value must be greater than 0"))
				return append(seq, badOp)
			}
		}
		var exprs []sem.SortExpr
		for _, e := range o.Exprs {
			exprs = append(exprs, t.sortExpr(nil, e, o.Reverse))
		}
		return append(seq, &sem.TopOp{
			Node:    o,
			Limit:   limit,
			Exprs:   exprs,
			Reverse: o.Reverse && len(exprs) == 0,
		})
	case *ast.PutOp:
		assignments := t.assignments(o.Args)
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
	case *ast.AssignmentOp:
		return append(seq, t.assignmentOp(o))
	case *ast.RenameOp:
		var assignments []sem.Assignment
		for _, fa := range o.Args {
			assign := t.assignment(&fa)
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
	case *ast.FuseOp:
		return append(seq, &sem.FuseOp{Node: o})
	case *ast.JoinOp:
		leftAlias, rightAlias := "left", "right"
		if o.Alias != nil {
			leftAlias = o.Alias.Left.Name
			rightAlias = o.Alias.Right.Name
		}
		if leftAlias == rightAlias {
			t.error(o.Alias, errors.New("left and right join aliases cannot be the same"))
			return append(seq, badOp)
		}
		var cond sem.Expr
		if o.Cond != nil {
			cond = t.pipeJoinCond(o.Cond, leftAlias, rightAlias)
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
		if len(seq) == 0 {
			seq = append(seq, &sem.PassOp{Node: join})
		}
		fork := &sem.ForkOp{
			Paths: []sem.Seq{
				seq,
				t.seq(o.RightInput),
			},
		}
		return sem.Seq{fork, join}
	case *ast.MergeOp:
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
			exprs = append(exprs, t.sortExpr(nil, e, false))
		}
		return append(seq, &sem.MergeOp{Node: o, Exprs: exprs})
	case *ast.UnnestOp:
		e := t.expr(o.Expr)
		t.enterScope()
		defer t.exitScope()
		var body sem.Seq
		if o.Body != nil {
			body = t.seq(o.Body)
		}
		return append(seq, &sem.UnnestOp{
			Node: o,
			Expr: e,
			Body: body,
		})
	case *ast.ShapesOp: // XXX move to std library?
		e := sem.Expr(sem.NewThis(o, nil))
		if o.Expr != nil {
			e = t.expr(o.Expr)
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
	case *ast.AssertOp:
		cond := t.expr(o.Expr)
		// 'assert EXPR' is equivalent to
		// 'values EXPR ? this : error({message: "assertion failed", "expr": EXPR_text, "on": this}'
		// where EXPR_text is the literal text of EXPR.
		return append(seq, sem.NewValues(o,
			&sem.CondExpr{
				Node: o.Expr,
				Cond: cond,
				Then: sem.NewThis(o, nil),
				Else: sem.NewCall(
					o.Expr,
					"error",
					[]sem.Expr{&sem.RecordExpr{
						Node: o.Expr,
						Elems: []sem.RecordElem{
							&sem.FieldElem{
								Node:  o.Expr,
								Name:  "message",
								Value: &sem.LiteralExpr{Node: o, Value: `"assertion failed"`},
							},
							&sem.FieldElem{
								Node:  o.Expr,
								Name:  "expr",
								Value: &sem.LiteralExpr{Node: o, Value: sup.QuotedString(o.Text)},
							},
							&sem.FieldElem{
								Node:  o.Expr,
								Name:  "on",
								Value: sem.NewThis(o.Expr, nil),
							},
						},
					}},
				),
			}))
	case *ast.ValuesOp:
		return append(seq, sem.NewValues(o, t.exprs(o.Exprs)...))
	case *ast.LoadOp:
		if !t.env.IsAttached() {
			t.error(o, errors.New("load operator cannot be used without an attached database"))
			return sem.Seq{badOp}
		}
		poolID, err := dbid.ParseID(o.Pool.Text)
		if err != nil {
			poolID, err = t.env.PoolID(t.ctx, o.Pool.Text)
			if err != nil {
				t.error(o, err)
				return append(seq, badOp)
			}
		}
		opArgs := t.opArgs(o.Args, "commit", "author", "message", "meta")
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
	case *ast.OutputOp:
		return append(seq, &sem.OutputOp{Node: o, Name: o.Name.Name})
	}
	panic(o)
}

func (t *translator) singletonAgg(assignment *ast.Assignment, seq sem.Seq) sem.Seq {
	if assignment.LHS != nil {
		return nil
	}
	out := t.assignment(assignment)
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

func (t *translator) singletonKey(agg ast.Assignment, seq sem.Seq) sem.Seq {
	if agg.LHS != nil {
		return nil
	}
	out := t.assignment(&agg)
	this, ok := out.LHS.(*sem.ThisExpr)
	if !ok || len(this.Path) != 1 {
		return nil
	}
	return append(seq,
		&sem.AggregateOp{
			Node: out.Node,
			Keys: []sem.Assignment{out},
		},
		sem.NewValues(out.Node, this),
	)
}

// semDecls enters a block of declarations into the current scope.  We do late binding
// of symbols to sem-tree entities so that the order of definition doesn't matter.
func (t *translator) decls(decls []ast.Decl) {
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.ConstDecl:
			t.constDecl(d)
		case *ast.FuncDecl:
			t.funcDecl(d)
		case *ast.OpDecl:
			t.opDecl(d)
		case *ast.PragmaDecl:
			t.pragmaDecl(d)
		case *ast.QueryDecl:
			t.queryDecl(d)
		case *ast.TypeDecl:
			t.typeDecl(d)
		default:
			panic(d)
		}
	}
}

type constDecl struct {
	decl    *ast.ConstDecl
	expr    sem.Expr
	scope   *Scope
	pending bool
}

func (c *constDecl) resolve(t *translator) sem.Expr {
	if c.expr == nil {
		if !c.pending {
			c.pending = true
			save := t.scope
			t.scope = NewScope(c.scope)
			defer func() {
				c.pending = false
				t.scope = save
			}()
			c.expr = t.mustEvalConst(t.expr(c.decl.Expr))
		} else {
			t.error(c.decl.Name, fmt.Errorf("const %q involved in cyclic dependency", c.decl.Name.Name))
			c.expr = badExpr
		}
	}
	return c.expr
}

func (t *translator) constDecl(c *ast.ConstDecl) {
	decl := &constDecl{
		decl:  c,
		scope: t.scope,
	}
	if err := t.scope.BindSymbol(c.Name.Name, decl); err != nil {
		t.error(c.Name, err)
	}
}

type queryDecl struct {
	decl    *ast.QueryDecl
	body    sem.Seq
	scope   *Scope
	pending bool
}

func (q *queryDecl) resolve(t *translator) sem.Seq {
	if q.body == nil {
		if !q.pending {
			save := t.scope
			q.pending = true
			t.scope = NewScope(q.scope)
			defer func() {
				q.pending = false
				t.scope = save
			}()
			q.body = t.seq(q.decl.Body)
		} else {
			t.error(q.decl.Name, fmt.Errorf("named query %q involved in cyclic dependency", q.decl.Name.Name))
			q.body = sem.Seq{badOp}
		}
	}
	return q.body
}

func (t *translator) queryDecl(q *ast.QueryDecl) {
	decl := &queryDecl{
		decl:  q,
		scope: t.scope,
	}
	if err := t.scope.BindSymbol(q.Name.Name, decl); err != nil {
		t.error(q.Name, err)
	}
}

func (t *translator) typeDecl(d *ast.TypeDecl) {
	typ, err := t.semType(d.Type)
	if err != nil {
		t.error(d.Type, err)
		typ = "null"
	}
	e := &sem.LiteralExpr{
		Node:  d.Name,
		Value: fmt.Sprintf("<%s=%s>", sup.QuotedName(d.Name.Name), typ),
	}
	val, ok := t.mustEval(e)
	if !ok {
		panic(e)
	}
	e.Value = sup.FormatValue(val)
	if err := t.scope.BindSymbol(d.Name.Name, e); err != nil {
		t.error(d.Name, err)
	}
}

func (t *translator) funcDecl(d *ast.FuncDecl) {
	funcDecl := t.resolver.newFuncDecl(d.Name.Name, d.Lambda, t.scope)
	if err := t.scope.BindSymbol(d.Name.Name, funcDecl); err != nil {
		t.error(d.Name, err)
	}
}

func (t *translator) opDecl(d *ast.OpDecl) {
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

func (t *translator) pragmaDecl(d *ast.PragmaDecl) {
	name := d.Name.Name
	if _, ok := t.scope.pragmas[name]; ok {
		t.error(d.Name, fmt.Errorf("%q redefined", name))
		return
	}
	expr := d.Expr
	if expr == nil {
		expr = &ast.Primitive{
			Kind: "Primitive",
			Type: "bool",
			Text: "true",
			Loc:  d.Name.Loc,
		}
	}
	switch name {
	case "index_base":
		if v := t.mustEvalPositiveInteger(expr); v <= 1 {
			t.scope.pragmas["index_base"] = v
		} else {
			t.error(d.Name, errors.New("index_base must be 0 or 1"))
		}
	case "pg":
		if v, ok := t.mustEvalBool(expr); ok {
			t.scope.pragmas["pg"] = v
		}
	default:
		t.error(d.Name, fmt.Errorf("unknown pragma %q", name))
	}
}

func (t *translator) assignmentOp(p *ast.AssignmentOp) sem.Op {
	var aggs, puts []sem.Assignment
	for _, astAssign := range p.Assignments {
		// Parition assignments into agg vs. puts.
		assign := t.assignment(&astAssign)
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
		return badOp
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

func (t *translator) exprOp(e ast.Expr, seq sem.Seq) sem.Seq {
	if call, ok := e.(*ast.CallExpr); ok {
		if seq := t.maybeCallShortcut(call, seq); seq != nil {
			return seq
		}
	} else if agg, ok := e.(*ast.AggFuncExpr); ok {
		return t.aggFuncShortcut(agg, seq)
	}
	// For stand-alone identifiers with no arguments, see if it's a user op
	// or a named query.
	if id, ok := e.(*ast.IDExpr); ok {
		if decl, err := t.scope.lookupOp(id.Name); err == nil {
			return append(seq, t.userOp(id.Loc, decl, nil)...)
		}
		if querySeq := t.scope.lookupQuery(t, id.Name); querySeq != nil {
			return append(seq, querySeq...)
		}
	}
	out := t.expr(e)
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
		if funcDef, ok := t.resolver.funcs[e.Tag]; ok {
			return t.isBool(funcDef.body)
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

func (t *translator) maybeCallShortcut(call *ast.CallExpr, seq sem.Seq) sem.Seq {
	f, ok := call.Func.(*ast.FuncNameExpr)
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

func (t *translator) aggFuncShortcut(agg *ast.AggFuncExpr, seq sem.Seq) sem.Seq {
	name := agg.Name
	aggFunc := t.aggFunc(agg, name, agg.Expr, agg.Filter, agg.Distinct)
	aggregate := &sem.AggregateOp{
		Node: agg,
		Aggs: []sem.Assignment{
			{
				Node: aggFunc,
				LHS:  sem.NewThis(agg, []string{name}),
				RHS:  aggFunc,
			},
		},
	}
	values := sem.NewValues(agg, sem.NewThis(agg, []string{name}))
	return append(seq, aggregate, values)
}

func (t *translator) callOp(call *ast.CallOp, seq sem.Seq) sem.Seq {
	decl, err := t.scope.lookupOp(call.Name.Name)
	if err != nil {
		t.error(call, err)
		return sem.Seq{badOp}
	}
	if decl == nil {
		t.error(call, fmt.Errorf("no such user operator: %q", call.Name.Name))
		return sem.Seq{badOp}
	}
	if decl.bad {
		return sem.Seq{badOp}
	}
	return append(seq, t.userOp(call.Loc, decl, call.Args)...)
}

func (t *translator) userOp(loc ast.Loc, decl *opDecl, args []ast.Expr) sem.Seq {
	// We've found a user op bound to the name being invoked, so we pull out the
	// AST elements that were stashed from the definition of the user op and subsitute
	// them into the call site here.  This is essentially a thunk... each use of the
	// user op is compiled into the context in which it appears and all the references
	// in that expression are bound appropriately with respect to this context.
	params := decl.ast.Params
	if len(params) != len(args) {
		t.error(loc, fmt.Errorf("%d arg%s provided when operator expects %d arg%s", len(args), plural.Slice(args, "s"), len(params), plural.Slice(params, "s")))
		return sem.Seq{badOp}
	}
	exprs := make([]sem.Expr, 0, len(args))
	for _, arg := range args {
		exprs = append(exprs, t.expr(arg))
	}
	if slices.Contains(t.opStack, decl.ast) {
		t.error(loc, opCycleError(append(t.opStack, decl.ast)))
		return sem.Seq{badOp}
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
			return sem.Seq{badOp}
		}
	}
	return t.seq(decl.ast.Body)
}

func (t *translator) opArgs(args []ast.OpArg, allowed ...string) opArgs {
	guard := make(map[string]struct{})
	for _, s := range allowed {
		guard[s] = struct{}{}
	}
	return t.opArgsAny(args, guard)
}

func (t *translator) opArgsAny(args []ast.OpArg, allowed map[string]struct{}) opArgs {
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
			opArgs[key] = argExpr{arg: arg, expr: t.expr(arg.Value)}
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

func (t *translator) mustEvalBool(in ast.Expr) (bool, bool) {
	val, ok := t.mustEval(t.expr(in))
	if ok {
		if super.TypeUnder(val.Type()) != super.TypeBool {
			t.error(in, errors.New("expected type bool"))
			return false, false
		}
		return val.AsBool(), true
	}
	return false, false
}

func (t *translator) maybeEvalString(e sem.Expr) (field string, ok bool) {
	val, ok := t.maybeEval(e)
	if ok && super.TypeUnder(val.Type()) == super.TypeString {
		return string(val.Bytes()), true
	}
	return "", false
}

func (t *translator) mustEvalPositiveInteger(ae ast.Expr) int {
	e := t.expr(ae)
	val, ok := t.mustEval(e)
	if !ok {
		return 0
	}
	if !super.IsInteger(val.Type().ID()) || val.IsNull() {
		t.error(ae, errors.New("expected integer"))
		return 0
	}
	v := int(val.AsInt())
	if v < 0 {
		t.error(ae, errors.New("expected positive integer"))
		return 0
	}
	return v
}

func (t *translator) mustEvalConst(e sem.Expr) sem.Expr {
	val, ok := t.mustEval(e)
	if !ok {
		return badExpr
	}
	return &sem.LiteralExpr{
		Node:  e,
		Value: sup.FormatValue(val),
	}
}

// mustEval leaves errors on the reporter and returns a bool as to whether
// the eval was successful
func (t *translator) mustEval(e sem.Expr) (super.Value, bool) {
	// We're in the middle of a semantic analysis but want to compile the
	// translated expression.  Operator thunks have been unfolded but
	// funcs haven't been resolved, but that's ok because we'll copy all the state
	// we need into a new instance of a translator (using the evaulator)
	// and we'll compile this all the way to a DAG and rungen it.  This is pretty
	// general because we need to handle things like subqueries that call
	// operator sequences that result in a constant value.
	return newEvaluator(t, t.resolver.funcs).mustEval(t.sctx, e)
}

// maybeEVal leaves no errors behind and simply returns a value and bool
// indicating if the eval was successful
func (t *translator) maybeEval(e sem.Expr) (super.Value, bool) {
	return newEvaluator(t, t.resolver.funcs).maybeEval(t.sctx, e)
}
