### Function

&emsp; **nameof** &mdash; the name of a named type

### Synopsis

```
nameof(val: any) -> string
```

### Description

The _nameof_ function returns the type name of `val` as a string if `val` is a named type.
Otherwise, it returns `error("missing")`.

### Examples

A named type yields its name and unnamed types yield a missing error:
```mdtest-command
echo '80(port=int16) 80' | super -z -c 'yield nameof(this)' -
```

```mdtest-output
"port"
error("missing")
```

The missing value can be ignored with quiet:
```mdtest-command
echo '80(port=int16) 80' | super -z -c 'yield quiet(nameof(this))' -
```

```mdtest-output
"port"
```
