### Function

&emsp; **sqrt** &mdash; square root of a number

### Synopsis
```
sqrt(val: number) -> float64
```

### Description
The _sqrt_ function returns the square root of its argument `val`, which
must be numeric.  The return value is a float64.  Negative values
result in `NaN`.

### Examples

The logarithm of a various numbers:
```mdtest-command
echo '4 2. 1e10 -1' | super -z -c 'yield sqrt(this)' -
```

```mdtest-output
2.
1.4142135623730951
100000.
NaN
```
