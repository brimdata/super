### Function

&emsp; **floor** &mdash; floor of a number

### Synopsis

```
floor(n: number) -> number
```

### Description

The _floor_ function returns the greatest integer less than or equal to its argument `n`,
which must be a numeric type.  The return type retains the type of the argument.

### Examples

The floor of a various numbers:
```mdtest-command
echo '1.5 -1.5 1(uint8) 1.5(float32)' | super -z -c 'yield floor(this)' -
```

```mdtest-output
1.
-2.
1(uint8)
1.(float32)
```
