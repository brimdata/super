# JOIN

A `JOIN` operation performs a relational join.

Joins are _conditional_ when they have the form
```
<table-expr> <join-type> JOIN <table-expr> <cond>
```
and are _non-conditional_ when having the form
```
<table-expr> <cross-join> <table-expr>
```
where
* `<table-expr>` is a [table expression](from.md#table-expressions)
  as defined in the [FROM](from.md) clause,
* `<join-type>` indicates the flavor of join as
  [described below](#join-types),
* `<cross-join>` is either a comma (`,`) or the keywords `CROSS JOIN`, and
* `<cond>` is the join condition in one of two forms:
  * `ON <predicate>` where `<predicate>` is a Boolean-valued
    [expression](../expressions/intro.md), or
  * `USING ( <id>, [ <id>, ... ] )` where `<id>` is an identifier indicating
     the one or more columns.

The `<table-expr>` on the left is called the _left table_ while the other
`<table-expr>` is the _right table_.  The two tables form a
[relational scope](intro.md#relational-scopes) called the _join scope_
consisting of the tables and columns from both tables.

Join operations are left associative and all of the join types have
equal precedence.

## Cross Join

A non-conditional join forms its output by combining each row in the
left table with all of the rows in the right table forming a cross product
between the two tables.  The order of the output rows is undefined.

## Conditional Join

Conditional joins logically form a cross join then filter the joined table
using the indicated join condition.

The join condition may be an `ON` clause or a `USING` clause.

The `ON <predicate>` clause applies the `<predicate>` to each combined row.
Table and column references within the `<predicate>` expression
are resolved using the [relational scope](intro.md#relational-scopes)
created by the left and right tables.

The `USING <id0> [ <id1>, ... ]` presumes each column is present in both
tables and applies an equality predicate for the indicated columns:
```
<L>.<id0> = <R>.<tid> AND <L>.<id1>=<R>.<id1>
```
where `<L>` and `<R>` are the names of the left and right tables.

### Join Types

For the `ON <predicate>` condition, the `<predicate>` is evaluated for
every row in the cross product and rows are included or excluded based
on the predicate's result as well as the `<join-type>`, which must be
one of:
* `LEFT [ OUTER ]` - produces an `INNER` join plus all rows in the left table
  not present in the inner join
* `RIGHT [ OUTER ]` - produces an `INNER` join plus all rows in the right table
  not present in the inner join
* `INNER` - produces the rows from the cross join that match the join condition,
* `ANTI` - produces the rows from the left table that are not in the inner join.

If no `<join-type>` is present, then an `INNER` join is presumed.

>[!NOTE]
> `FULL OUTER JOIN` is not yet supported by SuperSQL.  Also, note that
> `ANTI` is a left anti-join and there is no support for a right anti-join.

## Examples

---

_Inner join_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
),
U(z) AS (
    VALUES (2), (3)
)
SELECT *
FROM T
JOIN U ON x=z
# input

# expected output
{x:3,y:4,z:3}
```

---

_Left outer join_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
),
U(z) AS (
    VALUES (2), (3)
)
SELECT *
FROM T
LEFT JOIN U ON x=z
ORDER BY x
# input

# expected output
{x:1,y:2,z:error("missing")}
{x:3,y:4,z:3}
{x:5,y:6,z:error("missing")}
```

---

_Right outer join_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
),
U(z) AS (
    VALUES (2), (3)
)
SELECT *
FROM T
RIGHT JOIN U ON x=z
ORDER BY x
# input

# expected output
{x:3,y:4,z:3}
{x:error("missing"),y:error("missing"),z:2}
```

---

_Cross join_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
),
U(z) AS (
    VALUES (2), (3)
)
SELECT *
FROM T
CROSS JOIN U
ORDER BY z,y
# input

# expected output
{x:1,y:2,z:2}
{x:3,y:4,z:2}
{x:5,y:6,z:2}
{x:1,y:2,z:3}
{x:3,y:4,z:3}
{x:5,y:6,z:3}
```

---

_Inner join with USING condition_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
),
W(y) AS (
    VALUES (2), (3)
)
SELECT *
FROM T
JOIN W USING (y)
# input

# expected output
{y:2,x:1}
```

---
