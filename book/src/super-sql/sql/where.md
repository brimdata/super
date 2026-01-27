# WHERE

A `WHERE` clause has the form
```
WHERE <predicate>
```
where `<predicate>` is a Boolean-valued [expression](../expressions/index.md).

A WHERE clause is a component of [SELECT](select.md) that is applied
to the query's [input](from.md) removing each value from the input table
for which `<predicate>` is false.

The predicate may not contain any [aggregate functions](../aggregates/intro.md).

As in [PostgreSQL](https://www.postgresql.org/),
table and column references in the `WHERE` clause bind only to the
[input scope](intro.md#input-scope).

## Examples

---

_Filter on y while selecting x_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
)
SELECT x
FROM T
WHERE y >= 4
# input

# expected output
{x:3}
{x:5}
```

---

_A subquery in the WHERE clause_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
),
U(z) AS (
    VALUES (2), (3)
)
SELECT x
FROM T
WHERE y >= (SELECT MAX(z) FROM U)
# input

# expected output
{x:3}
{x:5}
```

---

_Cannot use aggregate functions in WHERE_
```mdtest-spq fails
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
)
SELECT x
FROM T
WHERE MIN(y) = 1
# input

# expected output
aggregate function "min" called in non-aggregate context at line 6, column 7:
WHERE MIN(y) = 1
      ~~~~~~
```

---
