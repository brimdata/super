### Function

&emsp; **coalesce** &mdash; return first value that is not null, a "missing" error, or a "quiet" error

### Synopsis

```
coalesce(val: any [, ... val: any]) -> any
```

### Description

The `coalesce` function returns the first of its arguments that is not null,
`error("missing")`, or `error("quiet")`.  It returns null if all its arguments
are null, `error("missing")`, or `error("quiet")`.

### Examples

---

```mdtest-spq
# spq
values coalesce(null, error("missing"), error("quiet"), this)
# input
1
# expected output
1
```

---

```mdtest-spq
# spq
values coalesce(null, error("missing"), this)
# input
error("quiet")
# expected output
null
```
