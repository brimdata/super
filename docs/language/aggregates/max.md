### Aggregate Function

&emsp; **max** &mdash; maximum value of input values

### Synopsis
```
max(number) -> number
```

### Description

The _max_ aggregate function computes the maximum value of its input.

### Examples

Maximum value of simple sequence:
```mdtest-spq
# spq
max(this)
# input
1 2 3 4
# expected output
4
```

Continuous maximum of simple sequence:
```mdtest-spq
# spq
yield max(this)
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

Unrecognized types are ignored:
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
