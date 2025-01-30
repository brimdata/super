---
weight: 4
title: Const, Func, Operator, and Type Statements
heading: Statements
---

## Const Statements

Constants may be defined and assigned to a symbolic name with the syntax
```
const <id> = <expr>
```
where `<id>` is an identifier and `<expr>` is a constant [expression](expressions.md)
that must evaluate to a constant at compile time and not reference any
runtime state such as `this`, e.g.,
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

One or more `const` statements may appear only at the beginning of a scope
(i.e., the main scope at the start of a query,
the start of the body of a [user-defined operator](#operator-statements),
or a [lateral scope](lateral-subqueries.md/#lateral-scope)
defined by an [`over` operator](operators/over.md))
and binds the identifier to the value in the scope in which it appears in addition
to any contained scopes.

A `const` statement cannot redefine an identifier that was previously defined in the same
scope but can override identifiers defined in ancestor scopes.

`const` statements may appear intermixed with `func` and `type` statements.

## Func Statements

User-defined functions may be created with the syntax
```
func <id> ( [<param> [, <param> ...]] ) : ( <expr> )
```
where `<id>` and `<param>` are identifiers and `<expr>` is an
[expression](expressions.md) that may refer to parameters but not to runtime
state such as `this`.

For example,
```mdtest-spq
# spq
func add1(n): (
  n+1
)

add1(this)
# input
1
2
3
4
# expected output
2
3
4
5
```

One or more `func` statements may appear at the beginning of a scope
(i.e., the main scope at the start of a query,
the start of the body of a [user-defined operator](#operator-statements),
or a [lateral scope](lateral-subqueries.md/#lateral-scope)
defined by an [`over` operator](operators/over.md))
and binds the identifier to the expression in the scope in which it appears in addition
to any contained scopes.

A `func` statement cannot redefine an identifier that was previously defined in the same
scope but can override identifiers defined in ancestor scopes.

`func` statements may appear intermixed with `const` and `type` statements.

## Operator Statements

User-defined operators may be created with the syntax

```
op <id> ( [<param> [, <param> ...]] ) : (
  <sequence>
)
```
where `<id>` is the operator identifier, `<param>` are the parameters for the
operator, and `<sequence>` is the chain of operators (e.g., `operator | ...`)
where the operator does its work.

A user-defined operator can then be called with using the familiar call syntax
```
<id> ( [<expr> [, <expr> ...]] )
```
where `<id>` is the identifier of the user-defined operator and `<expr>` is a list
of [expressions](expressions.md) matching the number of `<param>`s defined in
the operator's signature.

One or more `op` statements may appear only at the beginning of a scope
(i.e., the main scope at the start of a query,
the start of the body of a [user-defined operator](#operator-statements),
or a [lateral scope](lateral-subqueries.md/#lateral-scope)
defined by an [`over` operator](operators/over.md))
and binds the identifier to the value in the scope in which it appears in addition
to any contained scopes.

### Sequence `this` Value

The `this` value of a user-defined operator's sequence is provided by the
calling sequence.

For example,
```mdtest-spq
# spq
op myop(): (
  yield this
)

myop()
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
op AddMessage(field_for_message, msg): (
  field_for_message:=msg
)

AddMessage(message, "hello")
# input
{greeting: "hi"}
# expected output
{greeting:"hi",message:"hello"}
```

```mdtest-spq
# spq
op AddMessage(field_for_message, msg): (
  field_for_message:=msg
)

AddMessage(message, greeting)
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
op AddMessage(field_for_message, msg): (
  field_for_message:=msg
)

AddMessage("message", "hello")
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
op CountPool(pool_name): (
  from eval(pool_name) | count()
)

CountPool("example")
```

so that when we prepare and query the pool via
```mdtest-command
super db -q -lake test init
super db -q -lake test create -use example
echo '{greeting: "hello"}' | super db -q -lake test load -
super db -lake test query -z -I count-pool.spq
```

it produces the output
```mdtest-output
1(uint64)
```

### Nested Calls

User-defined operators can make calls to other user-defined operators that
are declared within the same scope or in a parent's scope. For example,
```mdtest-spq
# spq
op add1(x): (
  x := x + 1
)
op add2(x): (
  add1(x) | add1(x)
)
op add4(x): (
  add2(x) | add2(x)
)

add4(a.b)
# input
{a:{b:1}}
# expected output
{a:{b:5}}
```

One caveat with nested calls is that calls to other user-defined operators must
not produce a cycle, i.e., recursive and mutually recursive operators are not
allowed and will produce an error.

## Type Statements

Named types may be created with the syntax
```
type <id> = <type>
```
where `<id>` is an identifier and `<type>` is a [type](data-types.md#first-class-types).
This creates a new type with the given name in the type system, e.g.,
```mdtest-spq
# spq
type port=uint16

cast(this, <port>)
# input
80
# expected output
80(port=uint16)
```

One or more `type` statements may appear at the beginning of a scope
(i.e., the main scope at the start of a query,
the start of the body of a [user-defined operator](#operator-statements),
or a [lateral scope](lateral-subqueries.md/#lateral-scope)
defined by an [`over` operator](operators/over.md))
and binds the identifier to the type in the scope in which it appears in addition
to any contained scopes.

A `type` statement cannot redefine an identifier that was previously defined in the same
scope but can override identifiers defined in ancestor scopes.

`type` statements may appear intermixed with `const` and `func` statements.
