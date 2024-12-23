### Aggregate Function

&emsp; **min** &mdash; minimum value of input values

### Synopsis
```
min(...number) -> number
```

### Description

The _min_ aggregate function computes the minimum value of its input.

### Examples

Minimum value of simple sequence:
```mdtest-command
echo '1 2 3 4' | super -z -c 'min(this)' -
```

```mdtest-output
1
```

Continuous minimum of simple sequence:
```mdtest-command
echo '1 2 3 4' | super -z -c 'yield min(this)' -
```

```mdtest-output
1
1
1
1
```

Unrecognized types are ignored:
```mdtest-command
echo '1 2 3 4 "foo"' | super -z -c 'min(this)' -
```

```mdtest-output
1
```

Minimum value within buckets grouped by key:
```mdtest-command
echo '{a:1,k:1} {a:2,k:1} {a:3,k:2} {a:4,k:2}' |
  super -z -c 'min(a) by k |> sort' -
```

```mdtest-output
{k:1,min:1}
{k:2,min:3}
```
