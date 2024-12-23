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
```mdtest-command
echo '1 2 3 4' | super -z -c 'sum(this)' -
```

```mdtest-output
10
```

Continuous sum of simple sequence:
```mdtest-command
echo '1 2 3 4' | super -z -c 'yield sum(this)' -
```

```mdtest-output
1
3
6
10
```

Unrecognized types are ignored:
```mdtest-command
echo '1 2 3 4 "foo"' | super -z -c 'sum(this)' -
```

```mdtest-output
10
```

Sum of values bucketed by key:
```mdtest-command
echo '{a:1,k:1} {a:2,k:1} {a:3,k:2} {a:4,k:2}' |
  super -z -c 'sum(a) by k |> sort' -
```

```mdtest-output
{k:1,sum:3}
{k:2,sum:7}
```
