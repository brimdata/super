## Pipe Operators

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
produces incremental output will stream results as they are produced, i.e.,
running [`super`](../../command/super.md) to standard output
will display results incrementally.

The [`search`](search.md) and [`where`](where.md)
operators "find" values in their input and drop
the ones that do not match what is being looked for.

The [`values`](values.md) operator emits one or more output values
for each input value based on arbitrary [expressions](expressions.md),
providing a convenient means to derive arbitrary output values as a function
of each input value.

XXX rework refs to merge/combine

The [`fork` operator](fork.md) copies its input to parallel
branches of a pipeline.  The output of these parallel branches can be combined
in a number of ways:
* merged in sorted order using the [`merge` operator](operators/merge.md),
* joined using the [`join` operator](operators/join.md), or
* combined in an undefined order using the implied [`combine` operator](operators/combine.md).

A pipeline can also be split to multiple branches using the
[`switch` operator](switch.md), in which data is routed to only one
corresponding branch (or dropped) based on the switch clauses.

While the output order of the switch branches is undefined, order may be
reestablished by applying a [`sort`](sort.md) at the merge point of the `switch`.

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
