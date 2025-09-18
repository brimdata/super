### Function

&emsp; **map** &mdash; apply a function to each element of an array or set

### Synopsis

```
map(v: array|set, f: function) -> array|set|error
```

### Description

The `map` function applies a single-argument function `f`,
in the form of an existing function or a lambda expression,
to every element in array or set `v` and
returns an array or set of the results.

The function `f` may reference a [user function](../../statements.md#func-statements) or a built-in function using the `&` syntax as in
```
&<name>
```
where `<name>` is an identifier.

Alternatively, `f` may be a lambda expression of the form
```
lambda x: <expr>
```
where `<expr>` is any expression depending only on the lambda argument.

### Examples

---

_Upper case each element of an array using `&` for a built-in_
```mdtest-spq
# spq
values map(this, &upper)
# input
["foo","bar","baz"]
# expected output
["FOO","BAR","BAZ"]
```

---

_A user function to convert epoch floats to time values_
```mdtest-spq {data-layout="stacked"}
# spq
fn floatToTime(x): (
  cast(x*1000000000, <time>)
)
values map(this, &floatToTime)
# input
[1697151533.41415,1697151540.716529]
# expected output
[2023-10-12T22:58:53.414149888Z,2023-10-12T22:59:00.716528896Z]
```

---
_Same as above but with a lambda expression_

```mdtest-spq {data-layout="stacked"}
# spq
values map(this, lambda x:cast(x*1000000000, <time>))
# input
[1697151533.41415,1697151540.716529]
# expected output
[2023-10-12T22:58:53.414149888Z,2023-10-12T22:59:00.716528896Z]
```
