package semantic

import (
	"fmt"
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/runtime/sam/expr/function"
)

type funcParamLambda struct {
	param string // name of parameter that this instance of an argument is bound to
	id    string // decl ID (either built-in name or ID of decl slot)
}
type funcParamValue struct{}

type resolver struct {
	t     *translator
	decls map[string]*funcDecl // decl id to funcDecl
	funcs map[string]*funcDef  // call tag to funcDef
	fixed map[string]string    // decl id of fixed func to call tag
	ntag  int
}

func newResolver(t *translator) *resolver {
	return &resolver{
		t:     t,
		decls: make(map[string]*funcDecl),
		funcs: make(map[string]*funcDef),
		fixed: make(map[string]string),
	}
}

// There is a funcDef for every unique lambda-unraveled instance and
// exactly one for each such combination of decl IDs.  The instances
// are unqiue up to the lamba params, which are in turn identified by
// their decl ID (or built-in name).  Since lambda params originate only
// at a built-in or function declaration and they can't be modified
// (only passed by reference), the variants are determined by decl ID
// rather than call tag.  This allows us to translate and unravel
// all the functions and lambda arguments in a single pass integrated
// into the translator logic, which in turn, allows us to carry out
// type checking since functions are resolved to actual sem.Expr instances
// from the beginning.
type funcDef struct {
	tag     string
	name    string   // original name in decl or "lambda" (for errors)
	params  []string // params of args without any lambda args
	lambdas []lambda
	body    sem.Expr
}

type lambda struct {
	param string // parameter name this lambda arg appeared as
	pos   int    // position in the formal parameters list of the function
	id    string // declaration ID (or built-in) of the function value
}

func (r *resolver) resolveCall(n ast.Node, id string, args []sem.Expr) sem.Expr {
	if isBuiltin(id) {
		// Check argument count here for builtin functions.
		if _, err := function.New(super.NewContext(), id, len(args)); err != nil {
			r.t.error(n, err)
			return badExpr
		}
		return sem.NewCall(n, id, args)
	}
	return r.mustResolveCall(n, id, args)
}

func (r *resolver) mustResolveCall(n ast.Node, id string, args []sem.Expr) sem.Expr {
	d, ok := r.decls[id]
	if !ok {
		panic(id)
	}
	// Translate the decl ID to a func by converting any
	// function refs passed as args to a key into the variants table removing
	// correponding args.
	var params []string
	var exprs []sem.Expr
	declParams := d.lambda.Params
	if len(declParams) != len(args) {
		r.t.error(n, fmt.Errorf("%q: expected %d params but called with %d", d.name, len(declParams), len(args)))
		return badExpr
	}
	var lambdas []lambda
	for k, arg := range args {
		if f, ok := arg.(*sem.FuncRef); ok {
			lambdas = append(lambdas, lambda{param: declParams[k].Name, pos: k, id: f.ID})
			continue
		}
		if e, ok := arg.(*sem.ThisExpr); ok {
			if len(e.Path) == 1 {
				// Propagate a function passed as a function value inside of
				// a function to another function as the new param name.
				if id, ok := r.t.scope.lookupFuncParamLambda(e.Path[0]); ok {
					lambdas = append(lambdas, lambda{param: declParams[k].Name, pos: k, id: id})
					continue
				}
			}
		}
		params = append(params, declParams[k].Name)
		exprs = append(exprs, arg)
	}
	if len(declParams) == len(params) {
		// No need to specialize this call since no function args are being passed.
		return &sem.CallExpr{
			Node: n,
			Tag:  r.lookupFixed(id),
			Args: args,
		}
	}
	// Enter the new function scope and set up the bindings for the
	// values we retrieved above while evaluating args in the outer scope.
	return &sem.CallExpr{
		Node: n,
		Tag:  r.getVariant(id, params, lambdas).tag,
		Args: exprs,
	}
}

func (r *resolver) resolveVariant(d *funcDecl, variant *funcDef) {
	save := r.t.scope
	r.t.scope = NewScope(d.scope)
	defer func() {
		r.t.scope = save
	}()
	r.t.enterScope()
	for _, lambda := range variant.lambdas {
		r.t.scope.BindSymbol(lambda.param, &funcParamLambda{param: lambda.param, id: lambda.id})
	}
	for _, param := range variant.params {
		r.t.scope.BindSymbol(param, funcParamValue{})
	}
	variant.body = r.t.expr(d.lambda.Expr)
	r.t.exitScope()
}

func (r *resolver) resolveFixed(d *funcDecl, tag string) *funcDef {
	save := r.t.scope
	r.t.scope = NewScope(d.scope)
	defer func() {
		r.t.scope = save
	}()
	r.t.enterScope()
	params := idsToStrings(d.lambda.Params)
	for _, param := range params {
		r.t.scope.BindSymbol(param, funcParamValue{})
	}
	body := r.t.expr(d.lambda.Expr)
	r.t.exitScope()
	return &funcDef{
		tag:    tag,
		name:   d.name,
		params: params,
		body:   body,
	}
}

func (r *resolver) lookupFixed(id string) string {
	if tag, ok := r.fixed[id]; ok {
		return tag
	}
	tag := r.nextTag()
	// Install this binding up front to block recursion.
	r.fixed[id] = tag
	decl := r.decls[id]
	r.funcs[tag] = r.resolveFixed(decl, tag)
	return tag
}

func (r *resolver) lookupVariant(id string, lambdas []lambda) *funcDef {
	d := r.decls[id]
	for _, variant := range d.variants {
		if ok := matchVariant(variant, lambdas); ok {
			return variant
		}
	}
	return nil
}

func (r *resolver) getVariant(id string, params []string, lambdas []lambda) *funcDef {
	if variant := r.lookupVariant(id, lambdas); variant != nil {
		return variant
	}
	d := r.decls[id]
	tag := r.nextTag()
	variant := &funcDef{
		tag:     tag,
		name:    d.name,
		params:  params,
		lambdas: lambdas,
		// body needs to be resolved
	}
	r.funcs[tag] = variant
	// We put the variant in the decl before resolving the body to stop recursion.
	d.variants = append(d.variants, variant)
	// Resolve the body here.
	r.resolveVariant(d, variant)
	return variant
}

func matchVariant(def *funcDef, lambdas []lambda) bool {
	// find match or detect change and error or false/nil
	if len(def.lambdas) != len(lambdas) {
		return false
	}
	for k, lambda := range lambdas {
		if lambda.pos != def.lambdas[k].pos || lambda.id != def.lambdas[k].id {
			return false
		}
	}
	return true
}

type funcDecl struct {
	id       string
	name     string
	lambda   *ast.LambdaExpr
	scope    *Scope
	variants []*funcDef
}

func (r *resolver) newFuncDecl(name string, lambda *ast.LambdaExpr, s *Scope) *funcDecl {
	// decl IDs give us an id for sem.FuncRef, which can then be turned back
	// into a *funcDecl with r.decls
	id := strconv.Itoa(len(r.decls))
	r.decls[id] = &funcDecl{
		id:     id,
		name:   name,
		lambda: lambda,
		scope:  s,
	}
	return r.decls[id]
}

func (r *resolver) nextTag() string {
	tag := strconv.Itoa(r.ntag)
	r.ntag++
	return tag
}

func isBuiltin(tag string) bool {
	_, err := strconv.Atoi(tag)
	return err != nil
}
