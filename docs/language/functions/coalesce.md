### Function

&emsp; **coalesce** &mdash; return first value that is not null, a "missing" error, or a "quiet" error

### Synopsis

```
coalesce(val: any [, ... val: any]) -> bool
```

### Description

The _coalesce_ function returns the first of its arguments that is not null,
`error("missing")`, or `error("quiet")`.  It returns null if all its arguments
are null, `error("missing")`, or `error("quiet")`.

### Examples

```mdtest-command
super -z -c 'yield coalesce(null, error("missing"), error("quiet"), 1)'
```

```mdtest-output
1
```

```mdtest-command
super -z -c 'yield coalesce(null, error("missing"), error("quiet"))'
```

```mdtest-output
null
```
