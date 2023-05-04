---
sidebar_position: 3
sidebar_label: Func Statements
---

# Func Statements

User-defined functions may be created with the syntax
```
func <id> ( [<param> [, <param> ...]] ) : ( <expr> )
```
where `<id>` and `<param>` are identifiers and `<expr>` is an
[expression](expressions.md) that may refer to parameters but not to runtime
state such as `this`.

For example,
```mdtest-command
echo 1 2 3 4 | zq -z 'func add1(n): (n+1) add1(this)' -
```
produces
```mdtest-output
2
3
4
5
```

One or more `func` statements may appear at the beginning of a scope
(i.e., the main scope at the start of a Zed program or a [lateral scope](lateral-subqueries.md#lateral-scope)
defined by an [`over` operator](../operators/over.md))
and binds the identifier to the expression in the scope in which it appears in addition
to any contained scopes.

A `func` statement cannot redefine an identifier that was previously defined in the same
scope but can override identifiers defined in ancestor scopes.

`func` statements may appear intermixed with `const` and `type` statements.
