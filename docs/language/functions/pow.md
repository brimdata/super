### Function

&emsp; **pow** &mdash; exponential function of any base

### Synopsis

```
pow(x: number, y: number) -> float64
```

### Description

The _pow_ function returns the value `x` raised to the power of `y`.
The return value is a float64 or an error.

### Examples

```mdtest-spq
# spq
yield pow(this, 5)
# input
2
# expected output
32.
```
