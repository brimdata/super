### Function

&emsp; **round** &mdash; round a number

### Synopsis

```
round(val: number) -> number
```

### Description

The `round` function returns the number `val` rounded to the nearest integer value.
which must be a numeric type.  The return type retains the type of the argument.

### Examples

---

```mdtest-spq
# spq
values round(this)
# input
3.14
-1.5::float32
0
1
# expected output
3.
-2.::float32
0
1
```
