# ceil

ceiling of a number

## Synopsis

```
ceil(n: number) -> number
```

## Description

The `ceil` function returns the smallest integer greater than or equal to its argument `n`,
which must be a numeric type.  The return type retains the type of the argument.

## Examples

---

_The ceiling of various numbers_

```mdtest-spq
# spq
values ceil(this)
# input
1.5
-1.5
1::uint8
1.5::float32
# expected output
2.
-1.
1::uint8
2.::float32
```
