### Aggregate Function

&emsp; **sum** &mdash; sum of input values

### Synopsis
```
sum(number) -> number
```

### Description

The _sum_ aggregate function computes the mathematical sum of its input.

### Examples

Sum of simple sequence:
```mdtest-spq
# spq
sum(this)
# input
1
2
3
4
# expected output
10
```

Continuous sum of simple sequence:
```mdtest-spq
# spq
yield sum(this)
# input
1
2
3
4
# expected output
1
3
6
10
```

Unrecognized types are ignored:
```mdtest-spq
# spq
sum(this)
# input
1
2
3
4
127.0.0.1
# expected output
10
```

Sum of values bucketed by key:
```mdtest-spq
# spq
sum(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
{a:4,k:2}
# expected output
{k:1,sum:3}
{k:2,sum:7}
```
