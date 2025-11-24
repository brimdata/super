# LIMIT

A `LIMIT` clause has the form
```
LIMIT <count> [ OFFSET <skip> ]
```
or
```
OFFSET <skip> [ LIMIT <count> ]
```
where `<count>` and `<skip>` are numeric [expressions](../expressions/index.md)
that evaluate to compile time constants.

A `LIMIT` or `OFFSET` clause may appear after an `ORDER BY` clause or after
any [SQL operator](intro.md#sql-operator).

`LIMIT` may precede `OFFSET` or vice versa and the order is not significant.

`LIMIT` modifies the output of the preceding SQL operator by capping the number
of rows produced to `<count>`.  If the `OFFSET` clause is present,
then the first `<skip>` rows are ignored and the subsequent rows are produced
capping the output to `<count>` rows.

## Examples

---

_Reduce table from three rows to two_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,1), (2,2), (3,2)
)
SELECT x
FROM T
ORDER BY x
LIMIT 2
# input

# expected output
{x:1}
{x:2}
```

---

_Reduce table from three rows to two skipping the first row_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,1), (2,2), (3,2)
)
SELECT x
FROM T
ORDER BY x
OFFSET 1
LIMIT 2
# input

# expected output
{x:2}
{x:3}
```

---
