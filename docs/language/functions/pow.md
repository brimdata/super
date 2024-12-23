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

```mdtest-command
echo '2' | super -z -c 'yield pow(this, 5)' -
```

```mdtest-output
32.
```
