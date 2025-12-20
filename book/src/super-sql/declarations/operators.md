## Operators

New operators are declared with the syntax
```
op <name> [<param> [, <param> ...]] : (
  <query>
)
```
where
* `<name>` is an [identifier](../queries.md#identifiers)
  representing the name of the new operator,
* each `<param>` is an identifier
  representing a positional parameter to the operator, and
* `<query>` is any [query](../queries.md).

Operator declarations must appear in the declaration section of a [scope](../queries.md#scope).

### Call

A declared operator is invoked by its name
using the [call](../operators/intro.md#call) keyword.
Operators can be invoked without the `call` keyword as a shortcut when such use
is unambiguous with the [built-in operators](../operators/intro.md).

A called instance of a declared operator consumes input, operates on that input,
and produces output.  The body of the
operator declaration with argument expressions substituted into referenced parameters
defines how the input is processed.

An operator may also source its own data by beginning the query body
with a [from](../operators/from.md) operator or [SQL statement](../sql/intro.md).

### Nested Calls

Operators do not support recursion.  They cannot call themselves nor can they
form a mutually recursive dependency loop.
However, operators can call other operators whose declaration is in scope
as long as no dependency loop is formed.

### Closure-like Arguments

In contrast to function calls, where the arguments are evaluated at the call site
and values are passed to the function, operator arguments are instead passed to the
operator body as an expression _template_ and the expression is evaluated in the
context of the operator body.  That said, any other declared identifiers referenced
by these expressions (e.g., [constants](constants.md), [functions](functions.md), [named queries](queries.md), etc.) are bound to
those entities using the lexical scope where they are defined rather than the
scope of their expanded use in the operator's definition.

These expression arguments can be viewed as a
[closure](https://en.wikipedia.org/wiki/Closure_(computer_programming))
though there is no persistent state stored in the closure.
The [jq](https://github.com/jqlang/jq/wiki/jq-Language-Description#the-jq-language) language
describes its expression semantics as closures as well, though unlike jq,
the operator expressions here are not generators and do not implement backtracking.

### Examples

---

_Trivial operator that echoes its input, invoked with explicit `call`_

```mdtest-spq
# spq
op echo: (
  values this
)
call echo
# input
{x:1}
# expected output
{x:1}
```

---

_Simple example that adds a new field to input records_

```mdtest-spq
# spq
op decorate field, msg: (
  put field:=msg
)
decorate message, "hello"
# input
{greeting: "hi"}
# expected output
{greeting:"hi",message:"hello"}
```

---

_Error checking works as expected for non-l-values used as l-values_

```mdtest-spq fails {data-layout="stacked"}
# spq
op decorate field, msg: (
  put field:=msg
)
decorate 1, "hello"
# input
{greeting: "hi"}
# expected output
illegal left-hand side of assignment at line 2, column 7:
  put field:=msg
      ~~~~~~~~~~
```

---

_Nested calls_

```mdtest-spq
# spq
op add4 x: (
  op add2 x: (
    op add1 x: ( x:=x+1 )
    add1 x | add1 x
  )
  add2 x | add2 x
)
add4 a.b
# input
{a:{b:1}}
# expected output
{a:{b:5}}
```
