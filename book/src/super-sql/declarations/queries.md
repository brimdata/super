## Queries

A query may be bound to an identifier as a named query with the syntax
```
let <id> = ( <query> )
```
Named queries are similar to [common-table expressions (CTE)](../sql/with.md)
and are likwwise invoked in a [from](../operators/from.md) operator, as in
```
from <id>
```

XXX this is wrong (look at semantic).
Named queries must be standalone and not depend on any input.  Also, they are
based on dataflow scoping so, unlike CTEs, cannot refer to any relational variables
in a containing scope and thus cannot form correlated subqueries.

To declare a query that consumes the input where it occurs, you can instead define
an [operator](operators.md) that begins with a [values](../operator/values.md)
or [unnest](../operator/unnest.md) operator.

A common use case for a named query is to compute a complex query that returns a scalar
then embedding that scalar result in an expression.  Even though the named query
appears syntactically as a sub-query in this case, the result is efficient
because the compiler will materialize the result and reuse it on each invocation.
