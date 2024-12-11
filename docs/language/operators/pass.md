### Operator

&emsp; **pass** &mdash; copy input values to output

### Synopsis

```
pass
```
### Description

The `pass` operator outputs a copy of each input value. It is typically used
with operators that handle multiple branches of the pipeline such as
[`fork`](fork.md) and [`join`](join.md).

### Examples

_Copy input to output_
```mdtest-command
echo '1 2 3' | super -z -c pass -
```

```mdtest-output
1
2
3
```

_Copy each input value to three parallel pipeline branches and leave the values unmodified on one of them_
```mdtest-command
echo '"HeLlo, WoRlD!"' | super -z -c '
  fork (
    => pass
    => upper(this)
    => lower(this)
) |> sort' -
```

```mdtest-output
"HELLO, WORLD!"
"HeLlo, WoRlD!"
"hello, world!"
```
