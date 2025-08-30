### Function

&emsp; **sqrt** &mdash; square root of a number

### Synopsis
```
sqrt(val: number) -> float64
```

### Description

The `sqrt` function returns the square root of its argument `val`, which
must be numeric.  The return value is a float64.  Negative values
result in `NaN`.

### Examples

---

_The square root of various numbers_

```mdtest-spq
# spq
values sqrt(this)
# input
4
2.
1e10
-1
# expected output
2.
1.4142135623730951
100000.
NaN
```
