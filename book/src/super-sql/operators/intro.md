## Operators

The components of a SuperSQL [pipeline](../intro.md#pipe-queries)
are called pipe operators.  Each operator is identified by its name
and performs a specific operation on a sequence of values.

Some operators, like
[`aggregate`](aggregate.md) and [`sort`](sort.md),
read all of their input before producing output, though
`aggregate` can produce incremental results when the grouping key is
aligned with the order of the input.

For large queries that process all of their input, time may pass before
seeing any output.

On the other hand, most operators produce incremental output by operating
on values as they are produced.  For example, a long running query that
produces incremental output streams its results as produced, i.e.,
running [`super`](../../command/super.md) to standard output
will display results incrementally.

The [`search`](search.md) and [`where`](where.md)
operators "find" values in their input and drop
the ones that do not match what is being looked for.

The [`values`](values.md) operator emits one or more output values
for each input value based on arbitrary [expressions](../expressions.md),
providing a convenient means to derive arbitrary output values as a function
of each input value.

The [`fork`](fork.md) operator copies its input to parallel
branches of a pipeline, while the [`switch` operator](switch.md)
routes each input value to only one corresponding branch
(or drops the value) based on the switch clauses.

While the output order of parallel branches is [undefined](../intro.md#data-order),
order may be reestablished by applying a [`sort`](sort.md) at the merge point of
the `switch` or `fork`.

### Field Assignment

Several pipe operators manipulate records by modifying fields
or by creating new records from component expressions.

For example,

* the [`put`](put.md) operator adds or modifies fields,
* the [`cut`](cut.md) operator extracts a subset of fields, and
* the [`aggregate`](aggregate.md) operator forms new records from
[aggregate functions](../aggregates/intro.md) and grouping expressions.

In all of these cases, the SuperSQL language uses the syntax `:=` to denote
_field assignment_ and has the form:
```
<field> := <expr>
```

For example,
```
put x:=y+1
```
or
```
aggregate salary:=sum(income) by state:=lower(state)
```
This style of "assignment" to a record value is distinguished from the `=`
symbol, which denotes Boolean equality.

The field name and `:=` symbol may also be omitted and replaced with just the expression,
as in
```
aggregate count() by upper(key)
```
or
```
put lower(s), a.b.c, x+1
```
In this case, the field name is derived from the expression body as follows:
* for a dotted path expression, the name is the last element of the path;
* for a function or aggregate function, the name is the name of the function;
* for `this`, the name is `that`;
* otherwise, the name is the expression text formatted in a canonical form.

In the two examples above, the derived names are filled in as follows:
```
aggregate count:=count() by upper:=upper(key)
put lower:=lower(s), c:=a.b.c, `x+1`:=x+1
```

### Call

In addition to the built-in operators,
[new operators can be declared](../declarations/operators.md)
that take parameters and operate on input just like the built-ins. 

A declared operator is called using the `call` keyword:
```
call <id> [<arg> [, <arg> ...]]
```
where `<id>` is the name of the operator and each `<arg>` is an
[expression](../expressions/intro.md) or function reference.
The number of arguments must match the number
of parameters appearing in the operator declaration.

The `call` keyword is optional when the operator name does not
syntactically conflict with other operator syntax.

### Shortcuts

When interactively composing queries (e.g., within [SuperDB Desktop](https://zui.brimdata.io)),
it is often convenient to use syntactic shortcuts to quickly craft queries for
exploring data interactively as compared to a "coding style" of query writing.

Shortcuts allow certain operator names to be optionally omitted when
they can be inferred from context and are available for:
* [aggregate](aggregate.md),
* [put](put.md),
* [values](values.md), and
* [where](where.md).

For example, the SQL expression
```
SELECT count(),type GROUP BY type
```
is more concisely represented in pipe syntax as
```
aggregate count() by type
```
but even more succintly expressed as
```
count() by type
```
Here, the syntax of the [aggregate](operators/aggregate.md) is unambiguous
the `aggregate` keyword may be dropped.

Similary, an [expression](expressions.md) situated in the position
of a pipe operator implies a [values](values.md) shortcut, e.g.,
```
{a:x+1,b:y-1}
```
is shorthand for
```
values {a:x+1,b:y-1}
```
> _Note that the values shortcut means SuperSQL provides a calculator experience, e.g.,
> the command `super -c '1+1'` emits the value `2`._

When the expression is Boolean-valued, however, the shortcut is [where](where.md)
insetad of [values](values.md) providing a convenient means to filter values.
For example
```
x >= 1
```
is shorthand for
```
where x >= 1
```

Finally the [put](put.md) operator can be used as a shortcut where a list
of [field assignments](#field-assignment) may omit the `put` keyword.

For example, the operation
```
put a:=x+1,b:=y-1
```
can be expressed simply as
```
a:=x+1,b:=y-1
```
To confirm the interpretation of a shorcut, you can always check the compiler's
actions by running `super` with the `-C` flag to print the parsed query
in a "canonical form", e.g.,
```mdtest-command
super -C -c 'x >= 1'
super -C -c 'count() by type'
super -C -c '{a:x+1,b:y-1}'
super -C -c 'a:=x+1,b:=y-1'
```
produces
```mdtest-output
where x >= 1
aggregate
    count() by type
values {a:x+1,b:y-1}
put a:=x+1,b:=y-1
```
When composing long-form queries that are shared via SuperDB Desktop or managed in GitHub,
it is best practice to include all operator names in the source text.

### Operator Subqueries

TBD

An operator subquery is a query that is executed
in an iterated fashion over derived subsequences of the input
where the query runs to completion for each subsequence.

Currently, the only instance of an operator subquery
appears as the `into` clause of the [`unnest`](unnest.md) operator.

> _Future versions of SuperSQL will support other means to generate
> derived subsequences including window functions, data partitions, and so forth._

This pattern provides a means to generate a subquery evaluated on a different
sequence derived from each input value.  For example, `unnest` can be used 
to decompose the sequence 
```
{a:[1,2]}
{a:[3,4,5]}
```
into two subsequences `1,2` and `3,4,5`, and if we sum the
sequences using an associated subquery, then the output would be `3, 12`, e.g.,
```mdtest-spq
# spq
unnest a into ( sum(this) )
# input
{a:[1,2]}
{a:[3,4,5]}
# expected output
3
12
```
This result compares with the query `unnest a | sum(this)` which does not have
an operator subquery so all the values are combined into a single sequence and the
result is `15`.

> _This pattern rhymes with the SQL pattern of a "lateral
> join", which runs a subquery for each row of the outer query's results._

Quite naturally, operator subqueries may be nested as in
```mdtest-spq
# spq
unnest a into ( unnest this into ( sum(this) ) | collect(this) )
# input
{a:[[1,2]]}
{a:[[3,4,5],[6,7]]}
# expected output
[3]
[12,13]
```
Note that _any_ pipe operator sequence can appear in the body of the
operator subquery providing a powerful means to compose complex pipe queries.

Also, since any query can run as an
[expression subquery](../expressions/intro.md#expression-subqueries),
operator subqueries can run inside of expression subqueries, e.g.,
```mdtest-spq
# spq
values {
  smallest:(unnest a into (min(this))),
  biggest:(unnest a into (max(this)))
}
# input
{a:[1,2]}
{a:[3,4,5]}
# expected output
{smallest:1,biggest:2}
{smallest:3,biggest:5}
```
Of course, this query could also be written as
```
unnest a into (aggregate smallest:=min(this), biggest:=max(this))
```
