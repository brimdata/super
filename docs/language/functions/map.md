### Function

&emsp; **map** &mdash; apply a function to each element of an array or set

### Synopsis

```
map(v: array|set, f: function) -> array|set
```

### Description

The _map_ function applies function `f` to every element in array or set `v` and
returns an array or set of the results. Function `f` must be a function that takes
only one argument. `f` may be a [user-defined function](../statements.md#func-statements).

### Examples

Upper case each element of an array:

```mdtest-command
echo '["foo","bar","baz"]' | super -z -c 'yield map(this, upper)' -
```

```mdtest-output
["FOO","BAR","BAZ"]
```

Using a user-defined function to convert an epoch float to a time:

```mdtest-command
echo '[1697151533.41415,1697151540.716529]' |
  super -z -c '
    func floatToTime(x): (
      cast(x*1000000000, <time>)
    )
    yield map(this, floatToTime)
  ' -
```

```mdtest-output
[2023-10-12T22:58:53.414149888Z,2023-10-12T22:59:00.716528896Z]
```
