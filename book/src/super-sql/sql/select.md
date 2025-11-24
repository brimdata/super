## SELECT

A `SELECT` query is a `<query-body>` in the structure of a
[query envelope](intro.md#query-envelope).

Since a `<query-body>` is also a `<query-envelope>` and any
`<query-envelope>` is a [pipe operator](../operators/intro.md),
a `SELECT` query may be used anywhere a pipe operator may appear.

### Syntax

A `SELECT` query has the following structure:
```
SELECT [ DISTINCT | ALL ] <expr> [ AS <column> ] [ , <expr> [ AS <column> ]... ]
[ FROM <table-expr> [ , <table-expr> ... ] ]
[ WHERE <predicate> ]
[ GROUP BY <expr> [ , <expr> ... ]]
[ HAVING <predicate> ]
```
where
* `<expr>` is an [expression](../expressions/intro.md),
* `<column>` is an identifier, and
* `<predicate>` is a [Boolean-valued](../types/bool.md) expression.

As a `<query-body>`, a `SELECT` expression may be
[prefixed with](intro.md#query-envelope) a [WITH](with.md) clause
defining one or more CTEs and/or
[followed by](intro.md#query-envelope) optional
[ORDER BY](order.md) and [LIMIT](limit.md) clauses.

> [!NOTE]
> Grouping sets are not yet available in SuperSQL.

### The Projection

XXX explain selection semantics ref relational projection
