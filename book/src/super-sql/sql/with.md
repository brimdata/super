## WITH

XXX

The optional [WITH](with.md) clause may include one or more common-table expressions (CTE)
each of which binds a name to the query body defined in the CTE.
A CTE is similar to a [query declaration](../declarations/queries.md)
but the CTE body must be a `<query-envelope>` and the CTE name can be used
only with a [FROM](from.md) clause and is not accessible in an expression.
