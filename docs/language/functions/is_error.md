### Function

&emsp; **is_error** &mdash; test if a value is an error

### Synopsis

```
is_error(val: any) -> bool
```

### Description

The _is_error_ function returns true if its argument's type is error.
`is_error(v)` is shortcut for `kind(v)=="error"`,

### Examples

A simple value is not an error:
```mdtest-command
echo 1 | super -z -c 'yield is_error(this)' -
```

```mdtest-output
false
```

An error value is an error:
```mdtest-command
echo "error(1)" | super -z -c 'yield is_error(this)' -
```

```mdtest-output
true
```

Convert an error string into a record with an indicator and a message:
```mdtest-command
echo '"not an error" error("an error")' |
  super -z -c 'yield {err:is_error(this),message:under(this)}' -
```

```mdtest-output
{err:false,message:"not an error"}
{err:true,message:"an error"}
```
