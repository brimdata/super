# SELECT

A `SELECT` query has the form
```
SELECT [ DISTINCT | ALL ] <expr> [ AS <column> ] [ , <expr> [ AS <column> ]... ]
[ FROM <table-expr> [ , <table-expr> ... ] ]
[ WHERE <predicate> ]
[ GROUP BY <expr> [ , <expr> ... ]]
[ HAVING <predicate> ]
```
where
* `<expr>` is an [expression](../expressions/intro.md),
* `<column>` is an [identifier](../queries.md#identifiers),
* `<table-expr>` is an input as defined in the [FROM](from.md) clause, and
* `<predicate>` is a [Boolean-valued](../types/bool.md) expression.

As a [&lt;query-body>](intro.md#query-body), a `SELECT` may be
[prefixed by](intro.md#query-envelope) a [WITH](with.md) clause
defining one or more CTEs and/or
[followed by](intro.md#query-envelope) optional
[ORDER BY](order.md) and [LIMIT](limit.md) clauses.

A `SELECT` query may be used as a building block in more complex queries as it
is a [&lt;query-body>](intro.md#query-body) in the structure of a
[&lt;query envelope>](intro.md#query-envelope).

Since a `<query-body>` is also a `<query-envelope>` and any
`<query-envelope>` is a [pipe operator](../operators/intro.md),
a `SELECT` query may be used anywhere a pipe operator may appear.

> [!NOTE]
> Grouping sets are not yet available in SuperSQL.

## Input

* input scope
* grouping scope
* aggregate scope
* output scope

## Input Scope

The input to `SELECT` is a specified by its optional [FROM](from.md) clause,
which may combine inputs from multiple with [JOIN](join.md) clauses.


When `FROM` is not present, then the input is a single value `null`.

When present, the FROM clause provides an input to `SELECT`

## Projection

A ...
* forms a single input ...
* filters the input table

A `SELECT` query evaluates one or more [expressions](../expressions/intro.md)
to form a table comprised of named columns where the rows are represented
by a sequence of [records](../types/record.md).
The record fields correspond to the columns of the table
and the field names and positions are fixed over the entire
result set.  The type of a column may vary from row to row.

XXX explain selection semantics ref relational projection

The output... relational scope.

XXX need to explain aggregate function syntax somewhere as it is
different than pipe exprs because you can put aggfuncs in expressions.

