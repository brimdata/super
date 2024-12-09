### Function

&emsp; **log** &mdash; natural logarithm

### Synopsis

```
log(val: number) -> float64
```

### Description

The _log_ function returns the natural logarithm of its argument `val`, which
must be numeric.  The return value is a float64 or an error.

### Examples

The logarithm of various numbers:
```mdtest-command
echo '4 4.0 2.718 -1' | super -z -c 'yield log(this)' -
```

```mdtest-output
1.3862943611198906
1.3862943611198906
0.999896315728952
error({message:"log: illegal argument",on:-1})
```

The largest power of 10 smaller than the input:
```mdtest-command
echo '9 10 20 1000 1100 30000' | super -z -c 'yield int64(log(this)/log(10))' -
```

```mdtest-output
0
1
1
2
3
4
```
