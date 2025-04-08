### Function

&emsp; **upper** &mdash; convert a string to upper case

### Synopsis

```
upper(s: string) -> string
```

### Description

The _upper_ function converts all lower case Unicode characters in `s`
to upper case and returns the result.

### Examples

```mdtest-spq
# spq
yield upper(this)
# input
"Super format"
# expected output
"SUPER FORMAT"
```

[Slices](../expressions.md#slices) can be used to uppercase a subset of a string as well.

```mdtest-spq
# spq
func capitalize(str): (
  upper(str[1:2]) + str[2:]
)
yield capitalize(this)
# input
"super format"
# expected output
"Super format"
```
