### Aggregate Function

&emsp; **dcount** &mdash; count distinct input values

### Synopsis
```
dcount(any) -> uint64
```

### Description

The _dcount_ aggregation function uses hyperloglog to estimate distinct values
of the input in a memory efficient manner.

### Examples

Count of values in a simple sequence:
```mdtest-spq
# spq
dcount(this)
# input
1
2
2
3
# expected output
3(uint64)
```

Continuous count of simple sequence:
```mdtest-spq
# spq
yield dcount(this)
# input
1
2
2
3
# expected output
1(uint64)
2(uint64)
2(uint64)
3(uint64)
```

Mixed types are handled:
```mdtest-spq
# spq
yield dcount(this)
# input
1
"foo"
10.0.0.1
# expected output
1(uint64)
2(uint64)
3(uint64)
```

The estimated result may become less accurate with more unique input values:
```mdtest-command
seq 10000 | super -z -c 'dcount(this)' -
```
=>
```mdtest-output
9987(uint64)
```

Count of values in buckets grouped by key:
```mdtest-spq
# spq
dcount(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
# expected output
{k:1,dcount:2(uint64)}
{k:2,dcount:1(uint64)}
```
