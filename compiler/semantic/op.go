package semantic

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	 "github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/plural"
	"github.com/brimdata/super/pkg/reglob"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/zson"
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

func (a *analyzer) semFrom(from *ast.From, seq dag.Seq) dag.Seq {
	switch len(from.Trunks) {
	case 0:
		a.error(from, errors.New("from operator has no paths"))
		return append(seq, badOp())
	case 1:
		return a.semTrunk(from.Trunks[0], seq)
	default:
		paths := make([]dag.Seq, 0, len(from.Trunks))
		for _, in := range from.Trunks {
			paths = append(paths, a.semTrunk(in, nil))
		}
		return append(seq, &dag.Fork{
			Kind:  "Fork",
			Paths: paths,
		})
	}
}

func (a *analyzer) semTrunk(trunk ast.Seq, out dag.Seq) dag.Seq {
	src := trunk[0].(ast.Source)
	if pool, ok := src.(*ast.Pool); ok && len(trunk) > 1 {
		switch pool.Spec.Pool.(type) {
		case *ast.Glob, *ast.Regexp:
			a.error(src, errors.New("=> not allowed after pool pattern in 'from' operator"))
			return append(out, badOp())
		}
	}
	sources := a.semSource(src)
	seq := a.semSeq(trunk[1:])
	if len(sources) == 1 {
		return append(out, append(dag.Seq{sources[0]}, seq...)...)
	}
	paths := make([]dag.Seq, 0, len(sources))
	for _, source := range sources {
		paths = append(paths, append(dag.Seq{source}, seq...))
	}
	return append(out, &dag.Fork{Kind: "Fork", Paths: paths})
}

//XXX make sure you can't read files from a lake instance

func (a *analyzer) semOpSource(source ast.Source, out dag.Seq) dag.Seq {
	sources := a.semSource(source)
	if len(sources) == 1 {
		return append(sources, out...)
	}
	var paths []dag.Seq
	for _, s := range sources {
		paths = append(paths, dag.Seq{s})
	}
	return append(out, &dag.Fork{Kind: "Fork", Paths: paths})
}

func (a *analyzer) semSource(source ast.Source) []dag.Op {
	switch s := source.(type) {
	case *ast.File:
		var path string
		switch p := s.Path.(type) {
		case *ast.QuotedString:
			path = p.Text
		case *ast.String:
			// This can be either a reference to a constant or a string.
			var err error
			if path, err = a.maybeStringConst(p.Text); err != nil {
				a.error(s.Path, err)
			}
		default:
			panic(fmt.Errorf("semantic analyzer: unknown AST file type %T", p))
		}
		return []dag.Op{
			&dag.FileScan{
				Kind:     "FileScan",
				Path:     path,
				Format:   s.Format,
				SortKeys: a.semSortKeys(s.SortKeys),
			},
		}
	case *ast.HTTP:
		var headers map[string][]string
		if s.Headers != nil {
			expr := a.semExpr(s.Headers)
			val, err := kernel.EvalAtCompileTime(a.zctx, expr)
			if err != nil {
				a.error(s.Headers, err)
			} else {
				headers, err = unmarshalHeaders(val)
				if err != nil {
					a.error(s.Headers, err)
				}
			}
		}
		var url string
		switch p := s.URL.(type) {
		case *ast.QuotedString:
			url = p.Text
		case *ast.String:
			// This can be either a reference to a constant or a string.
			var err error
			if url, err = a.maybeStringConst(p.Text); err != nil {
				a.error(s.URL, err)
				// Set url so we don't report an error for this twice.
				url = "http://error"
			}
		default:
			panic(fmt.Errorf("semantic analyzer: unsupported AST get type %T", p))
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			a.error(s.URL, fmt.Errorf("invalid URL %s", url))
		}
		return []dag.Op{
			&dag.HTTPScan{
				Kind:     "HTTPScan",
				URL:      url,
				Format:   s.Format,
				SortKeys: a.semSortKeys(s.SortKeys),
				Method:   s.Method,
				Headers:  headers,
				Body:     s.Body,
			},
		}
	case *ast.Pool:
		if !a.source.IsLake() {
			a.error(s, errors.New("\"from pool\" cannot be used without a lake"))
			return []dag.Op{badOp()}
		}
		return a.semPool(s)
	case *ast.Pass:
		//XXX just connect parent
		return []dag.Op{dag.PassOp}
	case *ast.Delete:
		if !a.source.IsLake() {
			a.error(s, errors.New("deletions cannot be applied to non-lake entity"))
			return []dag.Op{badOp()}
		}
		pool, commit, err := a.checkHead("HEAD", "")
		if err != nil {
			a.error(s, err)
			return []dag.Op{badOp()}
		}
		poolID, commitID, err := a.lookupPoolIDs(pool, commit)
		if err != nil {
			a.error(s, err)
			return []dag.Op{badOp()}
		}
		return []dag.Op{
			&dag.DeleteScan{
				Kind:   "DeleteScan",
				ID:     poolID,
				Commit: commitID,
			},
		}
	default:
		panic(fmt.Errorf("semantic analyzer: unknown AST source type %T", s))
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

func (a *analyzer) semSortKeys(sortExprs []ast.SortExpr) order.SortKeys {
	var sortKeys order.SortKeys
	for _, e := range sortExprs {
		s := a.semSortExpr(e)
		switch key := s.Key.(type) {
		case *dag.This:
			sortKeys = append(sortKeys, order.NewSortKey(s.Order, key.Path))
		case *dag.BadExpr: // ignore so we don't report double errors
		default:
			a.error(e.Expr, errors.New("field required in sort expression"))
		}
	}
	return sortKeys
}

func (a *analyzer) semSortExpr(s ast.SortExpr) dag.SortExpr {
	e := a.semExpr(s.Expr)
	o := order.Asc
	if s.Order != nil {
		var err error
		if o, err = order.Parse(s.Order.Name); err != nil {
			a.error(s.Order, err)
		}
	}
	return dag.SortExpr{Key: e, Order: o}
}

func (a *analyzer) semPool(p *ast.Pool) []dag.Op {
	var poolNames []string
	var err error
	switch specPool := p.Spec.Pool.(type) {
	case nil:
		// This is a lake meta-query.
		poolNames = []string{""}
	case *ast.Glob:
		poolNames, err = a.matchPools(reglob.Reglob(specPool.Pattern), specPool.Pattern, "glob")
	case *ast.Regexp:
		poolNames, err = a.matchPools(specPool.Pattern, specPool.Pattern, "regexp")
	case *ast.String:
		// This can be either a reference to a constant or a string.
		var name string
		if name, err = a.maybeStringConst(specPool.Text); err == nil {
			poolNames = []string{name}
		}
	case *ast.QuotedString:
		poolNames = []string{specPool.Text}
	default:
		panic(fmt.Errorf("semantic analyzer: unknown AST pool type %T", specPool))
	}
	if err != nil {
		// XXX PoolSpec should have a node position but for now just report
		// error as entire source.
		a.error(p.Spec.Pool, err)
		return []dag.Op{badOp()}
	}
	var sources []dag.Op
	for _, name := range poolNames {
		sources = append(sources, a.semPoolWithName(p, name))
	}
	return sources
}

func (a *analyzer) maybeStringConst(name string) (string, error) {
	e, err := a.scope.LookupExpr(name)
	if err != nil || e == nil {
		return name, err
	}
	l, ok := e.(*dag.Literal)
	if !ok {
		return "", fmt.Errorf("%s: string value required", name)
	}
	val := zson.MustParseValue(a.zctx, l.Value)
	if val.Type().ID() != super.IDString {
		return "", fmt.Errorf("%s: string value required", name)
	}
	return val.AsString(), nil
}

func (a *analyzer) semPoolWithName(p *ast.Pool, poolName string) dag.Op {
	poolName, commit, err := a.checkHead(poolName, p.Spec.Commit)
	if err != nil {
		a.error(p.Spec.Pool, errors.New("cannot scan from unknown HEAD"))
		return badOp()
	}
	if poolName == "" {
		meta := p.Spec.Meta
		if meta == "" {
			a.error(p, errors.New("pool name missing"))
			return badOp()
		}
		if _, ok := dag.LakeMetas[meta]; !ok {
			a.error(p, fmt.Errorf("unknown lake metadata type %q in from operator", meta))
			return badOp()
		}
		return &dag.LakeMetaScan{
			Kind: "LakeMetaScan",
			Meta: p.Spec.Meta,
		}
	}
	poolID, commitID, err := a.lookupPoolIDs(poolName, commit)
	if err != nil {
		a.error(p.Spec.Pool, err)
		return badOp()
	}
	if meta := p.Spec.Meta; meta != "" {
		if _, ok := dag.CommitMetas[meta]; ok {
			if commitID == ksuid.Nil {
				commitID, err = a.source.CommitObject(a.ctx, poolID, "main")
				if err != nil {
					a.error(p, err)
					return badOp()
				}
			}
			return &dag.CommitMetaScan{
				Kind:   "CommitMetaScan",
				Meta:   meta,
				Pool:   poolID,
				Commit: commitID,
				Tap:    p.Spec.Tap,
			}
		}
		if _, ok := dag.PoolMetas[meta]; ok {
			return &dag.PoolMetaScan{
				Kind: "PoolMetaScan",
				Meta: meta,
				ID:   poolID,
			}
		}
		a.error(p, fmt.Errorf("unknown metadata type %q", meta))
		return badOp()
	}
	if commitID == ksuid.Nil {
		// This trick here allows us to default to the main branch when
		// there is a "from pool" operator with no meta query or commit object.
		commitID, err = a.source.CommitObject(a.ctx, poolID, "main")
		if err != nil {
			a.error(p, err)
			return badOp()
		}
	}
	return &dag.PoolScan{
		Kind:   "PoolScan",
		ID:     poolID,
		Commit: commitID,
	}
}

func (a *analyzer) checkHead(pool, commit string) (string, string, error) {
	if pool == "HEAD" {
		if a.head == nil {
			return "", "", errors.New("cannot scan from unknown HEAD")
		}
		pool = a.head.Pool
		commit = a.head.Branch
	}
	return pool, commit, nil
}

func (a *analyzer) lookupPoolIDs(pool, commit string) (ksuid.KSUID, ksuid.KSUID, error) {
	poolID, err := a.source.PoolID(a.ctx, pool)
	if err != nil {
		return ksuid.Nil, ksuid.Nil, err
	}
	var commitID ksuid.KSUID
	if commit != "" {
		if commitID, err = lakeparse.ParseID(commit); err != nil {
			commitID, err = a.source.CommitObject(a.ctx, poolID, commit)
			if err != nil {
				return ksuid.Nil, ksuid.Nil, err
			}
		}
	}
	return poolID, commitID, nil
}

func (a *analyzer) matchPools(pattern, origPattern, patternDesc string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	pools, err := a.source.Lake().ListPools(a.ctx)
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
	y := &dag.Yield{Kind: "Yield", Exprs: []dag.Expr{e}}
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
// object.  Currently, it only replaces the group-by duration with
// a bucket call on the ts and replaces FunctionCalls in op context
// with either a group-by or filter op based on the function's name.
func (a *analyzer) semOp(o ast.Op, seq dag.Seq) dag.Seq {
	switch o := o.(type) {
	case *ast.From:
		return a.semFrom(o, seq)
	case *ast.Pool, *ast.File, *ast.HTTP, *ast.Delete:
		return a.semOpSource(o.(ast.Source), seq)
	case *ast.Summarize:
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
		// coming from a parser AST, only from a worker using the kernel DSL,
		// which is another reason why we need separate parser and kernel ASTs.
		// Said another way, we don't want to do semantic analysis on a worker AST
		// as we presume that work had already been done and we just need
		// to execute it.  For now, the worker only uses a filter expression
		// so this code path isn't hit yet, but it uses this same entry point
		// and it will soon do other stuff so we need to put in place the
		// separation... see issue #2163.
		return append(seq, &dag.Summarize{
			Kind:  "Summarize",
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
		if _, err := super.NewRecordBuilder(a.zctx, fields); err != nil {
			a.error(o.Args, err)
			return append(seq, badOp())
		}
		return append(seq, &dag.Cut{
			Kind: "Cut",
			Args: assignments,
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
		for _, arg := range o.Args {
			sortExprs = append(sortExprs, a.semSortExpr(arg))
		}
		return append(seq, &dag.Sort{
			Kind:       "Sort",
			Args:       sortExprs,
			NullsFirst: o.NullsFirst,
			Reverse:    o.Reverse,
		})
	case *ast.Head:
		val := super.NewInt64(1)
		if o.Count != nil {
			expr := a.semExpr(o.Count)
			var err error
			if val, err = kernel.EvalAtCompileTime(a.zctx, expr); err != nil {
				a.error(o.Count, err)
				return append(seq, badOp())
			}
			if !super.IsInteger(val.Type().ID()) {
				a.error(o.Count, fmt.Errorf("expression value must be an integer value: %s", zson.FormatValue(val)))
				return append(seq, badOp())
			}
		}
		if val.AsInt() < 1 {
			a.error(o.Count, errors.New("expression value must be a positive integer"))
		}
		return append(seq, &dag.Head{
			Kind:  "Head",
			Count: int(val.AsInt()),
		})
	case *ast.Tail:
		val := super.NewInt64(1)
		if o.Count != nil {
			expr := a.semExpr(o.Count)
			var err error
			if val, err = kernel.EvalAtCompileTime(a.zctx, expr); err != nil {
				a.error(o.Count, err)
				return append(seq, badOp())
			}
			if !super.IsInteger(val.Type().ID()) {
				a.error(o.Count, fmt.Errorf("expression value must be an integer value: %s", zson.FormatValue(val)))
				return append(seq, badOp())
			}
		}
		if val.AsInt() < 1 {
			a.error(o.Count, errors.New("expression value must be a positive integer"))
		}
		return append(seq, &dag.Tail{
			Kind:  "Tail",
			Count: int(val.AsInt()),
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
		args := a.semExprs(o.Args)
		if len(args) == 0 {
			a.error(o, errors.New("no arguments given"))
		}
		limit := 1
		if o.Limit != nil {
			l := a.semExpr(o.Limit)
			val, err := kernel.EvalAtCompileTime(a.zctx, l)
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
		return append(seq, &dag.Top{
			Kind:  "Top",
			Args:  args,
			Flush: o.Flush,
			Limit: limit,
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
		rightInput := a.semSeq(o.RightInput)
		leftKey := a.semExpr(o.LeftKey)
		rightKey := leftKey
		if o.RightKey != nil {
			rightKey = a.semExpr(o.RightKey)
		}
		join := &dag.Join{
			Kind:     "Join",
			Style:    o.Style,
			LeftDir:  order.Unknown,
			LeftKey:  leftKey,
			RightDir: order.Unknown,
			RightKey: rightKey,
			Args:     a.semAssignments(o.Args),
		}
		if rightInput != nil {
			par := &dag.Fork{
				Kind:  "Fork",
				Paths: []dag.Seq{{dag.PassOp}, rightInput},
			}
			seq = append(seq, par)
		}
		return append(seq, join)
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
		return append(seq, &dag.Merge{
			Kind:  "Merge",
			Expr:  a.semExpr(o.Expr),
			Order: order.Asc, //XXX
		})
	case *ast.Over:
		if len(o.Locals) != 0 && o.Body == nil {
			a.error(o, errors.New("cannot have a with clause without a lateral query"))
		}
		a.enterScope()
		defer a.exitScope()
		locals := a.semVars(o.Locals)
		exprs := a.semExprs(o.Exprs)
		var body dag.Seq
		if o.Body != nil {
			body = a.semSeq(o.Body)
		}
		return append(seq, &dag.Over{
			Kind:  "Over",
			Defs:  locals,
			Exprs: exprs,
			Body:  body,
		})
	case *ast.Sample:
		e := dag.Expr(&dag.This{Kind: "This"})
		if o.Expr != nil {
			e = a.semExpr(o.Expr)
		}
		seq = append(seq, &dag.Summarize{
			Kind: "Summarize",
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
		return append(seq, &dag.Yield{
			Kind:  "Yield",
			Exprs: []dag.Expr{&dag.This{Kind: "This", Path: field.Path{"sample"}}},
		})
	case *ast.Assert:
		cond := a.semExpr(o.Expr)
		// 'assert EXPR' is equivalent to
		// 'yield EXPR ? this : error({message: "assertion failed", "expr": EXPR_text, "on": this}'
		// where EXPR_text is the literal text of EXPR.
		return append(seq, &dag.Yield{
			Kind: "Yield",
			Exprs: []dag.Expr{
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
									Value: &dag.Literal{Kind: "Literal", Value: zson.QuotedString([]byte(o.Text))},
								},
								&dag.Field{
									Kind:  "Field",
									Name:  "on",
									Value: &dag.This{Kind: "This"},
								},
							},
						}},
					},
				},
			},
		})
	case *ast.Yield:
		exprs := a.semExprs(o.Exprs)
		return append(seq, &dag.Yield{
			Kind:  "Yield",
			Exprs: exprs,
		})
	case *ast.Load:
		if !a.source.IsLake() {
			a.error(o, errors.New("load operator cannot be used without a lake"))
			return []dag.Op{badOp()}
		}
		poolID, err := lakeparse.ParseID(o.Pool)
		if err != nil {
			poolID, err = a.source.PoolID(a.ctx, o.Pool)
			if err != nil {
				a.error(o, err)
				return append(seq, badOp())
			}
		}
		return append(seq, &dag.Load{
			Kind:    "Load",
			Pool:    poolID,
			Branch:  o.Branch,
			Author:  o.Author,
			Message: o.Message,
			Meta:    o.Meta,
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
		&dag.Summarize{
			Kind: "Summarize",
			Aggs: []dag.Assignment{out},
		},
		&dag.Yield{
			Kind:  "Yield",
			Exprs: []dag.Expr{this},
		},
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
	if err := a.scope.DefineConst(a.zctx, c.Name, e); err != nil {
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
		Value: fmt.Sprintf("<%s=%s>", zson.QuotedName(d.Name.Name), typ),
	}
	if err := a.scope.DefineConst(a.zctx, d.Name, e); err != nil {
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
		funcs[i].Expr = a.semFuncBody(d, d.Params, d.Expr)
	}
	return funcs
}

func (a *analyzer) semFuncBody(d *ast.FuncDecl, params []*ast.ID, body ast.Expr) dag.Expr {
	a.enterScope()
	defer a.exitScope()
	for _, p := range params {
		if err := a.scope.DefineVar(p); err != nil {
			// XXX Each param should be a node but now just report the error
			// as the entire declaration.
			a.error(d, err)
		}
	}
	return a.semExpr(body)
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

func (a *analyzer) semVars(defs []ast.Def) []dag.Def {
	var locals []dag.Def
	for _, def := range defs {
		e := a.semExpr(def.Expr)
		if err := a.scope.DefineVar(def.Name); err != nil {
			a.error(def, err)
			continue
		}
		locals = append(locals, dag.Def{
			Name: def.Name.Name,
			Expr: e,
		})
	}
	return locals
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
	return &dag.Summarize{
		Kind: "Summarize",
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
	return append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: []dag.Expr{out},
	})
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
		summarize := &dag.Summarize{
			Kind: "Summarize",
			Aggs: []dag.Assignment{
				{
					Kind: "Assignment",
					LHS:  pathOf(name),
					RHS:  agg,
				},
			},
		}
		yield := &dag.Yield{
			Kind:  "Yield",
			Exprs: []dag.Expr{&dag.This{Kind: "This", Path: field.Path{name}}},
		}
		return append(append(seq, summarize), yield)
	}
	if !function.HasBoolResult(name) {
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
			val, err := kernel.EvalAtCompileTime(a.zctx, e)
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
				Value: zson.FormatValue(val),
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
