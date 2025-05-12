### Aggregate Function

&emsp; **min** &mdash; minimum value of input values

### Synopsis
```
min(number|string) -> number|string
```

### Description

The _min_ aggregate function computes the minimum value of its input.

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

Continuous minimum of simple numeric sequence:
```mdtest-spq
# spq
yield min(this)
# input
1
2
3
4
# expected output
1
1
1
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

Determining a minimum among mixed numeric and string inputs is not yet possible:
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
