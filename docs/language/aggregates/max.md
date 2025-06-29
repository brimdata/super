### Aggregate Function

&emsp; **max** &mdash; maximum value of input values

### Synopsis
```
max(number|string) -> number|string
```

### Description

The _max_ aggregate function computes the maximum value of its input.

When determining the _max_ of string inputs, values are compared via byte
order. This is equivalent to
[C/POSIX collation](https://www.postgresql.org/docs/current/collation.html#COLLATION-MANAGING-STANDARD)
as found in other SQL databases such as Postgres.

### Examples

Maximum value of simple numeric sequence:
```mdtest-spq
# spq
max(this)
# input
1
2
3
4
# expected output
4
```

Continuous maximum of simple numeric sequence:
```mdtest-spq
# spq
values max(this)
# input
1
2
3
4
# expected output
1
2
3
4
```

Maximum of several string values:
```mdtest-spq
# spq
max(this)
# input
"bar"
"foo"
"baz"
# expected output
"foo"
```

A mix of string and numeric input values results in an error:

```mdtest-spq
# spq
max(this)
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
max(this)
# input
1
2
3
4
127.0.0.1
# expected output
4
```

Maximum value within buckets grouped by key:
```mdtest-spq
# spq
max(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
{a:4,k:2}
# expected output
{k:1,max:2}
{k:2,max:4}
```
