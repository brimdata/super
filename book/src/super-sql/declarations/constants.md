## Constants

Constants may be defined and assigned to a name with the syntax
```
const <id> = <expr>
```
where `<id>` is an [identifier](../syntax.md#identifiers)
and `<expr>` is a constant [expression](expressions.md)
that must evaluate to a constant at compile time and not reference any
runtime state such as `this` or a field of `this`, e.g.,
```mdtest-spq
# spq
const PI=3.14159 2*PI*r
# input
{r:5}
{r:10}
# expected output
31.4159
62.8318
```
A constant can be any expression, inclusive of subqueries and function calls, as
long as the expression evalautes to a compile-time constant, e.g.,
```mdtest-spq
# spq
const ABC = [
  values 'a', 'b', 'c'
  | upper(this)
]
values ABC
# input
null
# expected output
["A","B","C"]
```

One or more `const` declarations may appear only at the beginning of a scope
(i.e., the main scope at the start of a query,
the start of the body of a [user-defined operator](#operator-statements),
or a [lateral scope](lateral-subqueries.md/#lateral-scope)
defined by an [`over` operator](operators/over.md))
and binds the identifier to the value in the scope in which it appears in addition
to any contained scopes.

A `const` statement cannot redefine an identifier that was previously defined in the same
scope but can override identifiers defined in ancestor scopes.

