## Functions

New functions are declared with the syntax
```
fn <id> ( [<param> [, <param> ...]] ) : <expr>
```
where `<id>` and `<param>` are identifiers.
`<id>` is the name of the new function and
each `<param>` names a positional argument of the function.

Constant declarations must appear in the declaration section of a [scope](../syntax.md#scope).

The function is defined by `<expr>`, which is any
[expression](../expressions/intro.md).
Thie function body may refer the passed-in arguments by name.

Specifically, the references to the named parameters are
field references of the special value `this`, as in any expression.
In particular, the value of `this` referenced in a function body
is formed as record from the actual values passed to the function
where the field names correspond to the parameters of the function.

For example, the function `add` as defined by
```
fn add(a,b): a+b
```
when invoked as
```
values {x:1} | values add(x,1)
```
is passed the record the `{a:x,b:1}`, which after resolving `x` to `1`,
is `{a:1,b:1}` and thus evaluates the expression
```
this.a + this.b
```
which results in `2`.

Any function-as-value arguments passed to a function do not appear in the `this`
record formed from the parameters.  Instead, function values are expanded at their
call sites in a macro-like fashion.

### Subquery Functions

Since the body of a function is any expression and an expression may be
a subquery, function bodies can be defined as [subqueries](../expressions/subqueries.md).
This leads to the commonly used pattern of a subquery function:
```
fn <id> ( [<param> [, <param> ...]] ) : (
    <query>
)
```
where `<query>` is any [query](../syntax.md) and is simply wrapped in parentheses
to form the subquery.

As with any subquery, when multiple results are expected, an array subquery
may be used by wrapping `<query>` in square brackets instead of parentheses:
```
fn <id> ( [<param> [, <param> ...]] ) : [
    <query>
]
```

### Examples

---

_A simple function that adds two numbers_

```mdtest-spq
# spq
fn add(a,b): a+b
values add(x,y)
# input
{x:1,y:2}
{x:2,y:2}
{x:3,y:3}
# expected output
3
4
6
```

---
