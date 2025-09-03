## Aggregate Functions

Aggregate functions appear in either the [`aggregate`](../operators/aggregate.md) operator
or in an [expression](../expressions.md).

When called within an expression, an output value is produced for every input value
using state from all values previously processed.
Because aggregate functions carry such state, their use can prevent the runtime
optimizer from parallelizing a query.

That said, aggregate function calls can be quite useful in a number of contexts.
For example, a unique ID can be assigned to the input quite easily:
```mdtest-spq
# spq
values {id:count(),value:this}
# input
"foo"
"bar"
"baz"
# expected output
{id:1::uint64,value:"foo"}
{id:2::uint64,value:"bar"}
{id:3::uint64,value:"baz"}
```

In contrast, calling aggregate functions from within the
[`aggregate`](../operators/aggregate.md) operator
produces just one output value.
```mdtest-spq {data-layout="stacked"}
# spq
aggregate count(),union(this)
# input
"foo"
"bar"
"baz"
# expected output
{count:3::uint64,union:|["bar","baz","foo"]|}
```
