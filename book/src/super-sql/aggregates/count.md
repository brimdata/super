### Aggregate Function

&emsp; **count** &mdash; count all input values

>[!TIP]
> For a running count as values arrive, see the [count](../operators/count.md) operator.

### Synopsis
```
count() -> int64
```

### Description

The _count_ aggregate function computes the number of values in its input.

### Examples

Count of values in a simple sequence:
```mdtest-spq
# spq
count()
# input
1
2
3
# expected output
3
```

Mixed types are handled:
```mdtest-spq
# spq
count()
# input
1
"foo"
10.0.0.1
# expected output
3
```

Count of values in buckets grouped by key:
```mdtest-spq
# spq
count() by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
# expected output
{k:1,count:2}
{k:2,count:1}
```

A simple count with no input values returns 0:
```mdtest-spq
# spq
where grep("bar", this) | count()
# input
1
"foo"
10.0.0.1
# expected output
0
```

Count can return an explicit zero when using a `filter` clause in the aggregation:
```mdtest-spq
# spq
count() filter (grep("bar", this))
# input
1
"foo"
10.0.0.1
# expected output
0
```

Note that the number of input values are counted, unlike the
[`len`](../functions/generics/len.md) function
which counts the number of elements in a given value:
```mdtest-spq
# spq
count()
# input
[1,2,3]
# expected output
1
```
