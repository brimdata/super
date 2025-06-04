### Function

&emsp; **abs** &mdash; absolute value of a number

### Synopsis

```
abs(n: number) -> number
```

### Description

The `abs` function returns the absolute value of its argument `n`, which
must be a numeric type.

### Examples

---

_Absolute value of various numbers_

```mdtest-spq {data-layout="stacked"}
# spq
values abs(this)
# input
1
-1
0
-1.0
-1::int8
1::uint8
"foo"
# expected output
1
1
0
1.
1::int8
1::uint8
error({message:"abs: not a number",on:"foo"})
```
