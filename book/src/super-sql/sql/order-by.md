# ORDER BY

An `ORDER BY` clause has the form
```
ORDER BY <sort-expr> [ , <sort-expr> ... ]
```
where each `<sort-expr>` has the form
```
<expr> | <ordinal> [ ASC | DESC] [ NULLS FIRST | NULLS LAST ]
```
`<expr>` is an [expression](../expressions/index.md) indicating
the sort key of the resulting order and `<ordinal>` is an expression
that evaluates to a compile-time constant integer indicating a column
number of the sorted table.

An `ORDER BY` clause may appear after any [SQL operator](intro.md#sql-operator)
and modifies the output of the preceding SQL operator by ordering the rows
by the value of `<expr>` or according to the column indicated by `<ordinal>`,
which is 1-based.

The `ASC` keyword indicates an ascending sort order while `DESC` indicates
descending.  If neither `ASC` or `DESC` is present, then `ASC` is presumed.

The `NULLS FIRST` keyword indicates that null values should appear first in
the sort order; otherwise they appear last.  If a `NULLS` clause is not
present, then `NULLS LAST` is presumed.

When the `ORDER BY` clause follows a [SELECT](select.md) operation,
the sort expressions are evaluated with respect to its
[input scope](intro.md#input-scope) and resolve identifiers
[column aliases](intro.md#column-aliases) at a precedence higher
than the input scope.

When the `ORDER BY` clause follows a SQL operator
that is not a SELECT operation, then sort expressions are evaluated
with respect to the [output scope](intro.md#output-scope)
created by that operator.

## Examples

---

_Sort on a column_
```mdtest-spq
# spq
SELECT x
ORDER BY x DESC
# input
{x:1,y:2}
{x:2,y:2}
{x:3,y:1}
# expected output
{x:3}
{x:2}
{x:1}
```

---

_Sort on two columns_
```mdtest-spq
# spq
SELECT x
ORDER BY y,x
# input
{x:1,y:2}
{x:2,y:2}
{x:3,y:1}
# expected output
{x:3}
{x:1}
{x:2}
```

---

_Sort on aggregate function_
```mdtest-spq
# spq
SELECT y
GROUP BY y
ORDER BY min(x)
# input
{x:1,y:2}
{x:2,y:2}
{x:3,y:1}
# expected output
{y:2}
{y:1}
```
