### Function

&emsp; **nullif** &mdash; returns a null value if values are equal

### Synopsis

```
nullif(val1: any, val2: any) -> any
```

### Description

The `nullif` function returns a `null` value if its first argument `val1` is
equal to its second argument `val2`, otherwise it returns `val1`.

### Examples

---

```mdtest-spq
# spq
nullif(1, 1)
# input

# expected output
null::int64
```

---

```mdtest-spq
# spq
nullif("foo", "bar")
# input

# expected output
"foo"
```
