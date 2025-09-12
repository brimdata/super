## Types

XXX types can be defined in a query in addition to in the input data

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
80::(port=uint16)
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
