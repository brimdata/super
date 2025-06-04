### Function

&emsp; **typeof** &mdash; the type of a value

### Synopsis

```
typeof(val: any) -> type
```

### Description

The `typeof` function returns the [type](../../types/intro.md) of
its argument `val`.  Types are first class so the returned type is
also a value.  The type of a type is type [`type`](../../types/type.md).

### Examples

---

_The types of various values_

```mdtest-spq
# spq
values typeof(this)
# input
1
"foo"
10.0.0.1
[1,2,3]
{s:"foo"}
null
error("missing")
# expected output
<int64>
<string>
<ip>
<[int64]>
<{s:string}>
<null>
<error(string)>
```

---

_The type of a type is type [`type`](../../types/type.md)_

```mdtest-spq
# spq
values typeof(typeof(this))
# input
null
# expected output
<type>
```
