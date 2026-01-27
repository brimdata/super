# GROUP BY

A `GROUP BY` clause has the form
```
GROUP BY <expr>|<ordinal> [ , <expr>|<ordinal> ... ]
```
where `<expr>` is an [expression](../expressions/index.md)
and `<ordinal>` is an expression
that evaluates to a compile-time constant integer indicating a column
number of the projection.

A GROUP BY clause is a component of [SELECT](select.md) that defines
the grouping logic for a [grouped projection](select.md#grouped-projection).

The expressions cause the input table's rows to be placed in groups,
one group for each unique value of the set of expressions present.
The table and column references in the grouping expressions bind
to the [input scope](intro.md#input-scope) and
[column aliases](intro.md#column-aliases).

When an `<ordinal>` is specified, the grouping expression is taken from the
projection's expressions with the leftmost column numbered 1 and so forth.

## Examples

---

_Compute an aggregate on x with grouping column y_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,1), (2,2), (3,2)
)
SELECT sum(x),y
FROM T
GROUP BY y
ORDER BY y
# input

# expected output
{sum:1,y:1}
{sum:5,y:2}
```

---

_Grouped table without an aggregate function_
```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,1), (2,2), (3,2)
)
SELECT (x+y)/3 as key
FROM T
GROUP BY (x+y)/3
ORDER BY key
# input

# expected output
{key:0}
{key:1}
```

---

_Group using projection column ordinal_

```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,1), (2,2), (3,2)
)
SELECT sum(x),y
FROM T
GROUP BY 2
ORDER BY y
# input

# expected output
{sum:1,y:1}
{sum:5,y:2}
```

---
