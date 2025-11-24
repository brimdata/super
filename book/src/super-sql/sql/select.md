# SELECT

A `SELECT` query has the form
```
SELECT [ DISTINCT | ALL ] <expr>|<star> [ AS <column> ] [ , <expr>|<star> [ AS <column> ]... ]
[ FROM <table-expr> [ , <table-expr> ... ] ]
[ WHERE <predicate> ]
[ GROUP BY <expr>|<ordinal> [ , <expr>|<ordinal> ... ]]
[ HAVING <predicate> ]
```
where
* `<expr>` is an [expression](../expressions/intro.md),
* `<star>` is a [column pattern](#column-patterns),
* `<column>` is an [identifier](../queries.md#identifiers),
* `<table-expr>` is an input as defined in the [FROM](from.md) clause,
* `<predicate>` is a [Boolean-valued](../types/bool.md) expression, and
* `<ordinal>` is a column number as defined in [GROUP BY](group-by.md).

The list of expressions followed the `SELECT` keyword is called
the _projection_ and the column names derived from the `AS` clauses
are referred to as the [_column aliases_](intro.md#column-aliases).

A `SELECT` query may be used as a building block in more complex queries as it
is a [&lt;sql-body>](intro.md#sql-body) in the structure of a
[&lt;sql-op>](intro.md#sql-operator).
Likewise, it may be
[prefixed by](intro.md#sql-operator) a [WITH](with.md) clause
defining one or more CTEs and/or
[followed by](intro.md#sql-operator) optional
[ORDER BY](order-by.md) and [LIMIT](limit.md) clauses.

Since a `<sql-body>` is also a `<sql-op>` and any
`<sql-op>` is a [pipe operator](../operators/intro.md),
a `SELECT` query may be used anywhere a pipe operator may appear.

> [!NOTE]
> Grouping sets are not yet available in SuperSQL.

## Execution Steps

A `SELECT` query performs its computation by
* forming an input table indicated by its [FROM](from.md) clause,
* optionally filtering the input table with its [WHERE](where.md) clause,
* optionally grouping rows into aggregates, one for each unique set of
  values of the grouping expressions specified by the [GROUP BY](group-by.md) clause, or grouping the entire input into a single aggregate row when
  there are [aggregate functions](../aggregates/intro.md) present,
* optionally filtering aggregated rows with its [HAVING](having.md) clause, and finally
* producing an output table based on the list of
  [expressions](../expressions/intro.md) or [column patterns](#column-patterns)
  in the `SELECT` clause.

A `SELECT` query typically specifies its input using one or more
tables specified in the [FROM](from.md) clause, but when the
`FROM` clause is omitted, the query takes its input from the
parent pipe operator.
If there is no parent operator and `FROM` is omitted, then the
default input is a single `null` value.

A `FROM` clause may also take input from its parent when using
an [f-string](../expressions/f-strings.md) as its input table.
In this case, the input table is dynamically typed.

## Column Patterns

A column pattern, as indicated by `<star>` above,
uses the `*` notation to match multiple columns
from the input table.  In its standalone form, it matches all columns
in the input table, e.g.,
```
SELECT * FROM table1 CROSS JOIN table2
```
matches all columns from `table1` and `table2`.

A column pattern may be prefixed with a table name as in `table.*` as in
```
SELECT table2.* FROM table1 CROSS JOIN table2
```
which matches only the columns from the specified table.

## The Projection

The output of the `SELECT` query, called the projection,
is a set of rows formed from the list of expressions following
the `SELECT` keyword where each rows is represented
by a [record](../types/record.md).
The record fields correspond to the columns of the table
and the field names and positions are fixed over the entire
result set.  The type of a column may vary from row to row when the
`SELECT` expressions produce values of varying types.

The names of the columns are specified by each `AS` clause.  When the
`AS` clause is absent, the column name is
[derived](../types/record.md#derived-field-names)
from the expression in the same way field names are derived from
expression in record expressions.

>[!NOTE] Column names currently must be unique as the underlying record
> type requires distinct field names.  Names are automatically deduplicated
> when there are conflicts.  SuperSQL will support duplicate
> column names in a future release.

The projection may be [grouped](#grouped-projection)
or [non-grouped](#non-grouped-projection).

### Grouped Projection

A grouped projection occurs when either or both occur:
* there is a [GROUP BY](group-by.md) clause, or
* there is at least one reference to an
  [aggregate function](../aggregates/intro.md) in the projection,
  in a `HAVING` clause, or in an `ORDER BY` clause.

In a grouped projection, the `HAVING` clause, `ORDER BY` clause, and
the projection may refer only to inputs that are aggregate functions
(where the function arguments are bound to the input scope and colum
aliases) or to expressions or combination of expressions that appear
in the `GROUP BY` clause.

Aggregate functions may be organized into expressions as any
other function but they may not appear anywhere inside of a
argument to another aggregate function.

There is one output row for each unique set of values of the
grouping expressions and the arguments for each instance of
each aggregate function are evaluated over the grouped set of values
optionally filtered with an aggregate function `FILTER` clause.

### Non-grouped Projection

A non-grouped projection occurs when there are no references to
aggregate functions and there is no `GROUP BY` clause.  In this case,
there cannot be a `HAVING` clause.

The projection formed here consists of the `SELECT` expressions
evaluated once for each row from the input table that is not
filtered by the `WHERE` clause.

## Examples

---

_Hello world_
```mdtest-spq
# spq
SELECT 'hello, world' AS message
# input

# expected output
{message:"hello, world"}
```

---

_Reference to `this` to see default input is null_

```mdtest-spq
# spq
SELECT this
# input

# expected output
{that:null}
```

---

_Mix alias and inferred column names_

```mdtest-spq
# spq
SELECT upper(s), upper(s[0:1])||s[1:] AS mixed
# input
{s:"foo"}
{s:"bar"}
# expected output
{upper:"FOO",mixed:"Foo"}
{upper:"BAR",mixed:"Bar"}
```

---

_Column names (currently) must be unique and are deduplicated_

```mdtest-spq
# spq
SELECT s, s
# input
{s:"foo"}
{s:"bar"}
# expected output
{s:"foo",s_1:"foo"}
{s:"bar",s_1:"bar"}
```

---

_Distinct values sorted_

```mdtest-spq
# spq
SELECT DISTINCT s ORDER BY s
# input
{s:"foo"}
{s:"bar"}
{s:"foo"}
# expected output
{s:"bar"}
{s:"foo"}
```

---

_Select entire rows as records using a table reference_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,1), (2,2), (3,2)
)
SELECT T
FROM T
# input

# expected output
{T:{x:1,y:1}}
{T:{x:2,y:2}}
{T:{x:3,y:2}}
```

---

_Select entire rows as records using `this`_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,1), (2,2), (3,2)
)
SELECT this as table
FROM T
# input

# expected output
{table:{x:1,y:1}}
{table:{x:2,y:2}}
{table:{x:3,y:2}}
```

---
