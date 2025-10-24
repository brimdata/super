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

Declared operators can then be called using the `call` keyword as in
```
call <id> [<arg> [, <arg> ...]]
```
where `<id>` is the name of the operator and each `<arg>` is an
[expression](../expressions/intro.md) or function reference.
The number of arguments must match the number
of parameters appearing in the operator declaration.

In contrast to function calls, where the arguments are evaluated at the call site
and values are passed to the function, operator arguments are instead passed to the
operator body as an expression _template_ and the expression is evaluated in the
context of the operator body.  That said, any other declared identifiers referenced
by these expressions (e.g., constants, functions, named queries, etc.) are bound to
those entities using the lexical scope of the use site rather than the operator body.

XXX But lexical scope is the use site.


### Sequence `this` Value

The `this` value of a user-defined operator's sequence is provided by the
calling sequence.

For example,
```mdtest-spq
# spq
op myop : (
  values this
)
myop
# input
{x:1}
# expected output
{x:1}
```

### Arguments

The arguments to a user-defined operator must be either constant values (e.g.,
a [literal](expressions.md#literals) or reference to a
[defined constant](#const-statements)), or a reference to a path in the data
stream (e.g., a [field reference](expressions.md#field-dereference)). Any
other expression will result in a compile-time error.

Because both constant values and path references evaluate in
[expression](expressions.md) contexts, a `<param>` may often be used inside of
a user-defined operator without regard to the argument's origin. For instance,
the `msg` parameter is used flexibly in the following examples.

```mdtest-spq
# spq
op AddMessage field_for_message, msg: (
  field_for_message:=msg
)
AddMessage message, "hello"
# input
{greeting: "hi"}
# expected output
{greeting:"hi",message:"hello"}
```

```mdtest-spq
# spq
op AddMessage field_for_message, msg: (
  field_for_message:=msg
)
AddMessage message, greeting
# input
{greeting: "hi"}
# expected output
{greeting:"hi",message:"hi"}
```
However, you may find it beneficial to use descriptive names for parameters
where _only_ a certain category of argument is expected. For instance, having
explicitly mentioned "field" in the name of our first parameter's name may help
us avoid making mistakes when passing arguments, such as
```mdtest-spq fails {data-layout="stacked"}
# spq
op AddMessage field_for_message, msg: (
  field_for_message:=msg
)
AddMessage "message", "hello"
# input
{greeting: "hi"}
# expected output
illegal left-hand side of assignment at line 2, column 3:
  field_for_message:=msg
  ~~~~~~~~~~~~~~~~~~~~~~
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
