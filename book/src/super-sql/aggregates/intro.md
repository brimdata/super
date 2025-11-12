## Aggregate Functions

Aggregate functions appear in the [`aggregate`](../operators/aggregate.md)
operator.

Calling aggregate functions from within the [`aggregate`](../operators/aggregate.md)
operator produces just one output value.
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
