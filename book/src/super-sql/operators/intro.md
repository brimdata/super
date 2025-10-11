## Operators

The components of a SuperSQL [pipeline](../intro.md#pipe-queries)
are called pipe operators.  Each operator is identified by its name
and performs a specific operation on a sequence of values.

XXX built-in and declared

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

In all of these cases, the SuperSQL language uses the token `:=` to denote
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
aggregate salary:=sum(income) by address:=lower(address)
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

XXX document how call works for declared operators

### Shortcuts

XXX TODO: clean up this section

XXX TODO: SuperSQL Shortcuts (change name here from implied operators)

XXX TODO: explain that expr shortcuts not allowed when ambiguous with
expressions (i.e., ({x:...} | ...) is ok but not ({x:...})

When SuperSQL is utilized in an application like [SuperDB Desktop](https://zui.brimdata.io),
queries are often composed interactively in a "search bar" experience.
The language design here attempts to support both this "lean forward" pattern of usage
along with a "coding style" of query writing where the queries might be large
and complex, e.g., to perform transformations in a data pipeline, where
the SuperSQL queries are stored under source-code control perhaps in GitHub.

To facilitate both a programming-like model as well as an ad hoc search
experience, SuperSQL has a canonical, long form that can be abbreviated
using syntax that supports an agile, interactive query workflow.
To this end, SuperSQL allows certain operator names to be optionally omitted when
they can be inferred from context.  For example, the expression following
the [`aggregate` operator](operators/aggregate.md)
```
aggregate count() by id
```
is unambiguously an aggregation and can be shortened to
```
count() by id
```
Likewise, a very common lean-forward use pattern is "searching", so with the
use of leading `?` shorthand, expressions are interpreted as keyword searches, e.g.,
```
search foo bar or x > 100
```
is abbreviated
```
? foo bar or x > 100
```
Furthermore, if an operator-free expression is not valid syntax for
a search expression but is a valid [expression](expressions.md),
then the abbreviation is treated as having an implied `yield` operator, e.g.,
```
{s:lower(s)}
```
is shorthand for
```
values {s:lower(s)}
```

Another common query pattern involves adding or mutating fields of records
where the input is presumed to be a sequence of records.
The [`put` operator](operators/put.md) provides this mechanism and the `put`
keyword is implied by the [field assignment](#field-assignments) syntax `:=`.

For example, the operation
```
put y:=2*x+1
```
can be expressed simply as
```
y:=2*x+1
```
When composing long-form queries that are shared via SuperDB Desktop or managed in GitHub,
it is best practice to include all operator names in the source text.

In summary, if no operator name is given, the implied operator is determined
from the operator-less source text, in the order given, as follows:
* If the text can be interpreted as a search expression and leading `?` shorthand is used, then the operator is `search`.
* If the text can be interpreted as a boolean expression, then the operator is `where`.
* If the text can be interpreted as one or more field assignments, then the operator is `put`.
* If the text can be interpreted as an aggregation, then the operator is `aggregate`.
* If the text can be interpreted as an expression, then the operator is `yield`.
* Otherwise, the text causes a compile-time error.

When in doubt, you can always check what the compiler is doing under the hood
by running `super` with the `-C` flag to print the parsed query in "canonical form", e.g.,
```mdtest-command
super -C -c '? foo'
super -C -c 'is(<foo>)'
super -C -c 'count()'
super -C -c '{a:x+1,b:y-1}'
super -C -c 'a:=x+1,b:=y-1'
```
produces
```mdtest-output
search foo
where is(<foo>)
aggregate
    count()
values {a:x+1,b:y-1}
put a:=x+1,b:=y-1
```

### Operator Subqueries

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
