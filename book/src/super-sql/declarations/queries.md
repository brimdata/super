## Queries

A query may be bound to an identifier as a named query with the syntax
```
let <name> = ( <query> )
```
where `<name>` is an [identifier](../queries.md#identifiers)
and `<query>` is any standalone [query](../queries.md) that sources its own input.

Named queries are similar to [common-table expressions (CTE)](../sql/with.md)
and may be likewise invoked in a [from](../operators/from.md) operator by referencing
the query's name, as in
```
from <name>
```
When invoked, a named query behaves like any query evaluated in the main scope
and receives a single `null` value as its input.  Thus, such queries typically
begin with a
[from](../operators/from.md)  or
[values](../operators/values.md) operator to provide explicit input.

Named queries can also be referenced within an expression, in which case they are
automatically invoked as an [expression subquery](../expressions/subqueries.md).
As with any expression subquery, multiple values result in an error, so when
this is expected, the query reference may be enclosed in brackets to form
an array subquery.

To create a query that can be used as an operator and thus can operate on its input,
declare an [operator](operators.md).

A common use case for a named query is to compute a complex query that returns a scalar,
then embedding that scalar result in an expression.  Even though the named query
appears syntactically as a sub-query in this case, the result is efficient
because the compiler will materialize the result and reuse it on each invocation.

### Examples

---

_Simplest named query_

```mdtest-spq
# spq
let hello = (values 'hello, world')
from hello
# input

# expected output
"hello, world"
```

---

_Use an array subquery if multiple values expected_

```mdtest-spq
# spq
let q = (values 1,2,3)
values [q]
# input

# expected output
[1,2,3]
```

---

_Assemble multiple query results into a record_

```mdtest-spq
# spq
let q1 = (values 1,2,3)
let q2 = (values 3,4)
let q3 = (values 5)
values {a:[q1],b:[q2],c:q3}
# input

# expected output
{a:[1,2,3],b:[3,4],c:5}
```
