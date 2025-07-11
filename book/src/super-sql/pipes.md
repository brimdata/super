# Pipes

TODO: clean this up, integrate better

When using pipes in SuperSQL, each operator takes its input from the output of its upstream operator beginning
either with a data source or with an implied source.

All available operators are listed on the [reference page](operators/_index.md).

XXX

## Pipeline Operators

Each operator is identified by name and performs a specific operation
on a stream of records.

Some operators, like
[`aggregate`](operators/aggregate.md) or [`sort`](operators/sort.md),
read all of their input before producing output, though
`aggregate` can produce incremental results when the grouping key is
aligned with the order of the input.

For large queries that process all of their input, time may pass before
seeing any output.

On the other hand, most operators produce incremental output by operating
on values as they are produced.  For example, a long running query that
produces incremental output will stream results as they are produced, i.e.,
running `super` to standard output will display results incrementally.

The [`search`](operators/search.md) and [`where`](operators/where.md)
operators "find" values in their input and drop
the ones that do not match what is being looked for.

The [`yield` operator](operators/yield.md) emits one or more output values
for each input value based on arbitrary [expressions](expressions.md),
providing a convenient means to derive arbitrary output values as a function
of each input value, much like the map concept in the MapReduce framework.

The [`fork` operator](operators/fork.md) copies its input to parallel
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
| merge val
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

## The Special Value `this`

In SuperSQL, there are no looping constructs and variables are limited to binding
values between [lateral scopes](lateral-subqueries.md#lateral-scope).
Instead, the input sequence
to an operator is produced continuously and any output values are derived
from input values.

In contrast to SQL, where a query may refer to input tables by name,
there are no explicit tables and an operator instead refers
to its input values using the special identifier `this`.

For example, sorting the following input produces the case-sensitive output
shown.
```mdtest-spq
# spq
sort
# input
"foo"
"bar"
"BAZ"
# expected output
"BAZ"
"bar"
"foo"
```

But we can make the sort case-insensitive by applying a [function](functions/_index.md) to the
input values with the expression `lower(this)`, which converts
each value to lower-case for use in in the sort without actually modifying
the input value, e.g.,

```mdtest-spq
# spq
sort lower(this)
# input
"foo"
"bar"
"BAZ"
# expected output
"bar"
"BAZ"
"foo"
```

## Implied Field References

XXX DISCARD (replaced by text in expressions section)

A common SuperSQL use case is to process sequences of record-oriented data
(e.g., arising from formats like JSON or Avro) in the form of events
or structured logs.  In this case, the input values to the operators
are [records](../formats/data-model.md#21-record) and the fields of a record are referenced with the dot operator.

For example, if the input above were a sequence of records instead of strings
and perhaps contained a second field, then we could refer to the field `s`
using `this.s` when sorting, which would give e.g.,
```mdtest-spq
# spq
sort this.s
# input
{s:"foo",x:1}
{s:"bar",x:2}
{s:"BAZ",x:3}
# expected output
{s:"BAZ",x:3}
{s:"bar",x:2}
{s:"foo",x:1}
```

This pattern is so common that field references to `this` may be shortened
by simply referring to the field by name wherever an expression is expected,
e.g., `sort s` is shorthand for `sort this.s`.

```mdtest-spq
# spq
sort s
# input
{s:"foo",x:1}
{s:"bar",x:2}
{s:"BAZ",x:3}
# expected output
{s:"BAZ",x:3}
{s:"bar",x:2}
{s:"foo",x:1}
```

## Field Assignments

A typical operation on records involves
adding or changing the fields of a record using the [`put` operator](operators/put.md)
or extracting a subset of fields using the [`cut` operator](operators/cut.md).
Also, when aggregating data using the [`aggregate` operator](operators/aggregate.md)
with grouping keys, the aggregate expressions create new named record fields.

In all of these cases, the SuperSQL language uses the token `:=` to denote
field assignment.  For example,
```
put x:=y+1
```
or
```
aggregate salary:=sum(income) by address:=lower(address)
```
This style of "assignment" to a record value is distinguished from the `=`
token which binds a locally scoped name to a value that can be referenced
in later expressions.

