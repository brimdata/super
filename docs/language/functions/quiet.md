### Function

&emsp; **quiet** &mdash; quiet "missing" errors

### Synopsis

```
quiet(val: any) -> any
```

### Description
The _quiet_ function returns its argument `val` unless `val` is
`error("missing")`, in which case it returns `error("quiet")`.
Various operators and functions treat quiet errors differently than
missing errors, in particular, dropping them instead of propagating them.
Quiet errors are ignored by operators `aggregate`, `cut`, and `values`.

### Examples

Values processes a quiet error and thus no output:
```mdtest-spq
# spq
values quiet(this)
# input
error("missing")
# expected output
```

Without quiet, values produces the missing error:
```mdtest-spq
# spq
values this
# input
error("missing")
# expected output
error("missing")
```

The `cut` operator drops quiet errors but retains missing errors:
```mdtest-spq
# spq
cut b:=x+1,c:=quiet(x+1),d:=quiet(a+1)
# input
{a:1}
# expected output
{b:error("missing"),d:2}
```
