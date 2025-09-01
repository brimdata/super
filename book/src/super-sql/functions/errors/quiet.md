### Function

&emsp; **quiet** &mdash; quiet "missing" errors

### Synopsis

```
quiet(val: any) -> any
```

### Description

The `quiet` function returns its argument `val` unless `val` is
`error("missing")`, in which case it returns `error("quiet")`.
Various operators and functions treat quiet errors differently than
missing errors, in particular, dropping them instead of propagating them.
Quiet errors are ignored by operators `aggregate`, `cut`, and `values`.

### Examples

---

_A quiet error in `values` produces no output_

```mdtest-spq
# spq
values quiet(this)
# input
error("missing")
# expected output
```

---

_Without quiet, values produces the missing error_

```mdtest-spq
# spq
values this
# input
error("missing")
# expected output
error("missing")
```

---

_The `cut` operator drops quiet errors but retains missing errors_

```mdtest-spq
# spq
cut b:=x+1,c:=quiet(x+1),d:=quiet(a+1)
# input
{a:1}
# expected output
{b:error("missing"),d:2}
```
