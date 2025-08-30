### Function

&emsp; **nameof** &mdash; the name of a named type

### Synopsis

```
nameof(val: any) -> string
```

### Description

The `nameof` function returns the type name of `val` as a string if `val` is a named type.
Otherwise, it returns `error("missing")`.

### Examples

---

_A named type yields its name and unnamed types values a missing error_

```mdtest-spq
# spq
values nameof(this)
# input
80::(port=int16)
80
# expected output
"port"
error("missing")
```

---

_The missing value can be ignored with quiet_

```mdtest-spq
# spq
values quiet(nameof(this))
# input
80::(port=int16)
80
# expected output
"port"
```
