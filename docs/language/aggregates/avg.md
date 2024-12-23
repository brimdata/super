### Aggregate Function

&emsp; **avg** &mdash; average value

### Synopsis
```
avg(number) -> number
```

### Description

The _avg_ aggregate function computes the mathematical average value of its input.

### Examples

Average value of simple sequence:
```mdtest-command
echo '1 2 3 4' | super -z -c 'avg(this)' -
```

```mdtest-output
2.5
```

Continuous average of simple sequence:
```mdtest-command
echo '1 2 3 4' | super -z -c 'yield avg(this)' -
```

```mdtest-output
1.
1.5
2.
2.5
```

Unrecognized types are ignored:
```mdtest-command
echo '1 2 3 4 "foo"' | super -z -c 'avg(this)' -
```

```mdtest-output
2.5
```

Average of values bucketed by key:
```mdtest-command
echo '{a:1,k:1} {a:2,k:1} {a:3,k:2} {a:4,k:2}' |
  super -z -c 'avg(a) by k |> sort' -
```

```mdtest-output
{k:1,avg:1.5}
{k:2,avg:3.5}
```
