### Function

&emsp; **round** &mdash; round a number

### Synopsis

```
round(val: number) -> number
```

### Description

The _round_ function returns the number `val` rounded to the nearest integer value.
which must be a numeric type.  The return type retains the type of the argument.

### Examples

```mdtest-command
echo '3.14 -1.5 0 1' | super -z -c 'yield round(this)' -
```

```mdtest-output
3.
-2.
0
1
```
