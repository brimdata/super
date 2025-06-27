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
```mdtest-spq
# spq
values map(this, upper)
# input
["foo","bar","baz"]
# expected output
["FOO","BAR","BAZ"]
```

Using a user-defined function to convert epoch floats to time values:
```mdtest-spq {data-layout="stacked"}
# spq
func floatToTime(x): (
  cast(x*1000000000, <time>)
)
values map(this, floatToTime)
# input
[1697151533.41415,1697151540.716529]
# expected output
[2023-10-12T22:58:53.414149888Z,2023-10-12T22:59:00.716528896Z]
```
