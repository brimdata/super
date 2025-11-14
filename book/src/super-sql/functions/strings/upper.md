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

_Simple call on upper_

```mdtest-spq
# spq
values upper(this)
# input
"Super format"
# expected output
"SUPER FORMAT"
```

---

_Apply upper to a string [slice](../../expressions/slices.md)_

```mdtest-spq
# spq
fn capitalize(str): (
  upper(str[0:1]) + str[1:]
)
values capitalize(this)
# input
"super format"
# expected output
"Super format"
```
