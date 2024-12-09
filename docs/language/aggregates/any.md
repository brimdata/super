### Aggregate Function

&emsp; **any** &mdash; select an arbitrary input value

### Synopsis
```
any(any) -> any
```

### Description

The _any_ aggregate function returns an arbitrary element from its input.
The semantics of how the item is selected is not defined.

### Examples

Any picks the first one in this scenario but this behavior is undefined:
```mdtest-command
echo '1 2 3 4' | super -z -c 'any(this)' -
```

```mdtest-output
1
```

Continuous any over a simple sequence:
```mdtest-command
echo '1 2 3 4' | super -z -c 'yield any(this)' -
```

```mdtest-output
1
1
1
1
```

Any is not sensitive to mixed types as it just picks one:
```mdtest-command
echo '"foo" 1 2 3 ' | super -z -c 'any(this)' -
```

```mdtest-output
"foo"
```

Pick from groups bucketed by key:
```mdtest-command
echo '{a:1,k:1} {a:2,k:1} {a:3,k:2} {a:4,k:2}' |
  super -z -c 'any(a) by k |> sort' -
```

```mdtest-output
{k:1,any:1}
{k:2,any:3}
```
