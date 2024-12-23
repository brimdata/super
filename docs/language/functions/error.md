### Function

&emsp; **error** &mdash; wrap a Zed value as an error

### Synopsis

```
error(val: any) -> error
```

### Description

The _error_ function returns an error version of any value.
It wraps the value `val` to turn it into an error type providing
a means to create structured and stacked errors.

### Examples

Wrap a record as a structured error:
```mdtest-command
echo '{foo:"foo"}' |
  super -z -c 'yield error({message:"bad value", value:this})' -
```

```mdtest-output
error({message:"bad value",value:{foo:"foo"}})
```

Wrap any value as an error:
```mdtest-command
echo '1 "foo" [1,2,3]' | super -z -c 'yield error(this)' -
```

```mdtest-output
error(1)
error("foo")
error([1,2,3])
```

Test if a value is an error and show its type "kind":
```mdtest-command
echo 'error("exception") "exception"' |
  super -Z -c 'yield {this,err:is_error(this),kind:kind(this)}' -
```

```mdtest-output
{
    this: error("exception"),
    err: true,
    kind: "error"
}
{
    this: "exception",
    err: false,
    kind: "primitive"
}
```

Comparison of a missing error results in a missing error even if they
are the same missing errors so as to not allow field comparisons of two
missing fields to succeed:
```mdtest-command
echo '{}' | super -z -c 'badfield:=x |> yield badfield==error("missing")' -
```

```mdtest-output
error("missing")
```
