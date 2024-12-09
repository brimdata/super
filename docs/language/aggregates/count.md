### Aggregate Function

&emsp; **count** &mdash; count input values

### Synopsis
```
count() -> uint64
```

### Description

The _count_ aggregate function computes the number of values in its input.

### Examples

Count of values in a simple sequence:
```mdtest-command
echo '1 2 3' | super -z -c 'count()' -
```

```mdtest-output
3(uint64)
```

Continuous count of simple sequence:
```mdtest-command
echo '1 2 3' | super -z -c 'yield count()' -
```

```mdtest-output
1(uint64)
2(uint64)
3(uint64)
```

Mixed types are handled:
```mdtest-command
echo '1 "foo" 10.0.0.1' | super -z -c 'yield count()' -
```

```mdtest-output
1(uint64)
2(uint64)
3(uint64)
```

Count of values in buckets grouped by key:
```mdtest-command
echo '{a:1,k:1} {a:2,k:1} {a:3,k:2}' | super -z -c 'count() by k |> sort' -
```

```mdtest-output
{k:1,count:2(uint64)}
{k:2,count:1(uint64)}
```

A simple count with no input values returns no output:
```mdtest-command
echo '1 "foo" 10.0.0.1' | super -z -c 'where grep("bar") |> count()' -
```

```mdtest-output
```

Count can return an explicit zero when using a `where` clause in the aggregation:
```mdtest-command
echo '1 "foo" 10.0.0.1' | super -z -c 'count() where grep("bar")' -
```

```mdtest-output
0(uint64)
```

Note that the number of input values are counted, unlike the [`len` function](../functions/len.md) which counts the number of elements in a given value:
```mdtest-command
echo '[1,2,3]' | super -z -c 'count()' -
```

```mdtest-output
1(uint64)
```
