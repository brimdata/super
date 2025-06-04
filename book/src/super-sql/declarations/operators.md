## Operators

New operators are declared with the syntax
```
op <id> [<param> [, <param> ...]] : (
  <query>
)
```
where 
* `<id>` is an identifier representing the name of the new operator,
* each `<param>` is an identifier representing a positional parameter to the operator, and
* `<query>` is any [query](../syntax.md).

Operator declarations must appear in the declaration section of a [scope](../syntax.md#scope).

A declared operator is invoked called using the [call](../operators/intro.md#call) keyword.
Operators can be invoked without the `call` keyword as a shortcut when such use
is unambiguous with the built-in operators.

A called instance of a declared operator consumes input, operates on that input,
and produces output.  The body of the
operator declaration with argument expressions substituted into referenced parameters
defines how the input is processed.

An operator may also source its own data by beginning the query body
with a [from](../operators/from.md) operator or [SQL statement](../sql/intro.md).

Operators do not support recursion.  They cannot call themselves nor can they
form a mutually recursive dependency loop.

In contrast to function calls, where the arguments are evaluated at the call site
and values are passed to the function, operator arguments are instead passed to the
operator body as an expression _template_ and the expression is evaluated in the
context of the operator body.  That said, any other declared identifiers referenced
by these expressions (e.g., constants, functions, named queries, etc.) are bound to
those entities using the lexical scope of the use site rather than the lexical
scope of the operator body's definition.

The expression arguments can be viewed as a
[closure](https://en.wikipedia.org/wiki/Closure_(computer_programming))
though there is no persistent state stored in the closure.
The [jq](https://github.com/jqlang/jq/wiki/jq-Language-Description#the-jq-language) language
describes its expression semantics as closures as well, though unlike jq,
the operator expressions here are not generators and do not implement backtracking.

### Examples

---

_Trivial operator that echoes its input_

```mdtest-spq
# spq
op echo: (
  values this
)
echo
# input
{x:1}
# expected output
{x:1}
```

---

_Simple example that adds a new field to inputs records_

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
Error: illegal left-hand side of assignment at line 2, column 7:
  put field:=msg
      ~~~~~~~~~~
```

A constant value must be used to pass a parameter that will be referenced as
the data source of a [`from` operator](operators/from.md). For example, we
quote the pool name in our program `count-pool.spq`
```mdtest-input count-pool.spq
op CountPool pool_name: (
  from eval(pool_name) | count()
)
CountPool "example"
```
so that when we prepare and query the pool via
```mdtest-command
super db -q -db test init
super db -q -db test create -use example
echo '{greeting: "hello"}' | super db -q -db test load -
super db -db test -s -I count-pool.spq
```

it produces the output
```mdtest-output
1::uint64
```

### Nested Calls

User-defined operators can make calls to other user-defined operators that
are declared within the same scope or in a parent's scope. For example,
```mdtest-spq
# spq
op add1 x: (
  x := x + 1
)
op add2 x: (
  add1 x | add1 x
)
op add4 x: (
  add2 x | add2 x
)
add4 a.b
# input
{a:{b:1}}
# expected output
{a:{b:5}}
```
One caveat with nested calls is that calls to other user-defined operators must
not produce a cycle, i.e., recursive and mutually recursive operators are not
allowed and will produce an error.
