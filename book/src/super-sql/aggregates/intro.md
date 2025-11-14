## Aggregate Functions

Aggregate functions compute aggregated results from zero or more
input values and have the form
```
<name> ( [ all | distinct ] <expr> ) [ where <pred> ]
```
where
* `<name>` is an identifier naming the function,
* `all` and `distinct` are optional keywords,
* `<expr>` is any [expression](../expressions/intro.md) that is type compatible
with the particular function, and
* `<pred>` is an optional Boolean expression that filters inputs to the function.

Aggregate functions may appear in
* the [aggregate](../operators/aggregate.md) operator,
* an aggregate [shortcut](../operators/intro.md#shortcuts), or
* in [SQL operators](../sql/intro.md) when performing aggregations.

When aggregate functions appear in context of grouping
(e.g., the `by` clause of an [aggregate](../operators/aggregate.md) operator or a
[SQL operator](../sql/intro.md) with a [GROUP BY](../sql/group-by.md) clause),
then the aggregate function produces one output value for each
unique combination of grouping expressions.

If the case-insensitive `distinct` option is present, then the inputs
to the aggregate function are filtered to unique values for each
unique grouping.

If the case-insensitive `all` option is present, then all values
for each unique group are passed as input to the aggregate function.

Calling an aggregate function in a pipe-operator expression outside of
an aggregating context is an error.
