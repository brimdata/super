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
Quiet errors are ignored by operators `cut`, `summarize`, and `yield`.

### Examples

Yield processes a quiet error and thus no output:
```mdtest-command
echo 'error("missing")' | super -z -c 'yield quiet(this)' -
```

```mdtest-output
```

Without quiet, yield produces the missing error:
```mdtest-command
echo 'error("missing")' | super -z -c 'yield this' -
```

```mdtest-output
error("missing")
```

The `cut` operator drops quiet errors but retains missing errors:
```mdtest-command
echo '{a:1}' | super -z -c 'cut b:=x+1,c:=quiet(x+1),d:=quiet(a+1)' -
```

```mdtest-output
{b:error("missing"),d:2}
```
