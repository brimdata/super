## Aggregate Function Calls

[Aggregate functions](../aggregates/intro.md) may be called within an expression.

Unlike the aggregation context provided by the [`aggregate` operator](../operators/aggregate.md),
such calls in expression context values an output value for each input value.

Note that because aggregate functions carry state which is typically
dependent on the order of input values, their use can prevent the runtime
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

In contrast, calling aggregate functions within the [`aggregate` operator](operators/aggregate.md)
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
