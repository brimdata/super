### Function

&emsp; **upper** &mdash; convert a string to upper case

### Synopsis

```
upper(s: string) -> string
```

### Description

The `upper` function converts all lower case Unicode characters in `s`
to upper case and returns the result.

### Examples

---

```mdtest-spq
# spq
values upper(this)
# input
"Super format"
# expected output
"SUPER FORMAT"
```

[Slices](../../expressions.md#slices) can be used to uppercase a subset of a string as well.

```mdtest-spq
# spq
fn capitalize(str): (
  upper(str[1:2]) + str[2:]
)
values capitalize(this)
# input
"super format"
# expected output
"Super format"
```
