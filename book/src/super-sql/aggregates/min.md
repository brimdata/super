### Aggregate Function

&emsp; **min** &mdash; minimum value of input values

### Synopsis
```
min(number|string) -> number|string
```

### Description

The _min_ aggregate function computes the minimum value of its input.

When determining the _min_ of string inputs, values are compared via byte
order. This is equivalent to
[C/POSIX collation](https://www.postgresql.org/docs/current/collation.html#COLLATION-MANAGING-STANDARD)
as found in other SQL databases such as Postgres.

### Examples

Minimum value of simple numeric sequence:
```mdtest-spq
# spq
min(this)
# input
1
2
3
4
# expected output
1
```

Minimum of several string values:
```mdtest-spq
# spq
min(this)
# input
"foo"
"bar"
"baz"
# expected output
"bar"
```

A mix of string and numeric input values results in an error:
```mdtest-spq
# spq
min(this)
# input
1
"foo"
2
# expected output
error("mixture of string and numeric values")
```

Other unrecognized types in mixed input are ignored:
```mdtest-spq
# spq
min(this)
# input
1
2
3
4
127.0.0.1
# expected output
1
```

Minimum value within buckets grouped by key:
```mdtest-spq
# spq
min(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
{a:4,k:2}
# expected output
{k:1,min:1}
{k:2,min:3}
```
