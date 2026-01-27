# HAVING

A `HAVING` clause has the form
```
HAVING <predicate>
```
where `<predicate>` is a Boolean-valued [expression](../expressions/index.md).

A HAVING clause is a component of [SELECT](select.md) that is applied
to the query's grouped output removing each value from the input table
for which `<predicate>` is false.

The predicate cannot refer to the input scope except for expressions
whose components are grouping expressions or aggregate functions whose
arguments refer to the input scope.

## Examples

---
_Simple aggregate without GROUP BY_
```mdtest-spq
# spq
SELECT 'hello, world' as message
HAVING count()=1
# input

# expected output
{message:"hello, world"}
```
---

_HAVING referencing the grouping expression_

```mdtest-spq
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
)
SELECT min(y)
FROM T
GROUP BY (x+y)/7
HAVING (x+y)/7=0
# input

# expected output
{min:2}
```
---

_HAVING clause without a grouped output is an error_
```mdtest-spq fails
# spq
WITH T(x,y) AS (
    VALUES (1,2), (3,4), (5,6)
)
SELECT x
FROM T
HAVING y >= 4
# input

# expected output
HAVING clause requires aggregation functions and/or a GROUP BY clause at line 6, column 8:
HAVING y >= 4
       ~~~~~~
```

---