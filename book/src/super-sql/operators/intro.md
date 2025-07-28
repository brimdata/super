## Pipe Operators

Each component of a SuperSQL [pipeline](../intro.md/#pipe-queries)
is a pipe operator and each operator is identified by its name
and performs a specific operation on a sequence of values.

Some operators, like
[`aggregate`](operators/aggregate.md) and [`sort`](operators/sort.md),
read all of their input before producing output, though
`aggregate` can produce incremental results when the grouping key is
aligned with the order of the input.

For large queries that process all of their input, time may pass before
seeing any output.

On the other hand, most operators produce incremental output by operating
on values as they are produced.  For example, a long running query that
produces incremental output will stream results as they are produced, i.e.,
running `super` to standard output will display results incrementally.

The [`search`](search.md) and [`where`](where.md)
operators "find" values in their input and drop
the ones that do not match what is being looked for.

The [`values`](values.md) operator emits one or more output values
for each input value based on arbitrary [expressions](expressions.md),
providing a convenient means to derive arbitrary output values as a function
of each input value.

The [`fork` operator](fork.md) copies its input to parallel
branches of a pipeline.  The output of these parallel branches can be combined
in a number of ways:
* merged in sorted order using the [`merge` operator](operators/merge.md),
* joined using the [`join` operator](operators/join.md), or
* combined in an undefined order using the implied [`combine` operator](operators/combine.md).

A pipeline can also be split to multiple branches using the
[`switch` operator](operators/switch.md), in which data is routed to only one
corresponding branch (or dropped) based on the switch clauses. For example:

```mdtest-spq
# spq
switch this
  case 1 ( values {val:this,message:"one"} )
  case 2 ( values {val:this,message:"two"} )
  default ( values {val:this,message:"many"} )
| sort
# input
1
2
3
4
# expected output
{val:1,message:"one"}
{val:2,message:"two"}
{val:3,message:"many"}
{val:4,message:"many"}
```
Note that the output order of the switch branches is undefined (indeed they run
in parallel on multiple threads).  To establish a consistent sequence order,
a [`merge` operator](operators/merge.md)
may be applied at the output of the `switch` specifying a sort key upon which
to order the upstream data.  Often such order does not matter (e.g., when the output
of the switch hits an [aggregator](aggregates/_index.md)), in which case it is typically more performant
to omit the merge (though the SuperDB runtime will often delete such unnecessary
operations automatically as part optimizing queries when they are compiled).

If no `merge` or `join` is indicated downstream of a `fork` or `switch`,
then the implied `combine` operator is presumed.  In this case, values are
forwarded from the switch to the downstream operator in an undefined order.

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
