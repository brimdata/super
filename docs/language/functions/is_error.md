### Function

&emsp; **is_error** &mdash; test if a value is an error

### Synopsis

```
is_error(val: any) -> bool
```

### Description

The _is_error_ function returns true if its argument's type is error.
`is_error(v)` is shorthand for `kind(v)=="error"`,

### Examples

A simple value is not an error:
```mdtest-spq
# spq
yield is_error(this)
# input
1
# expected output
false
```

An error value is an error:
```mdtest-spq
# spq
yield is_error(this)
# input
error(1)
# expected output
true
```

Convert an error string into a record with an indicator and a message:
```mdtest-spq
# spq
yield {err:is_error(this),message:under(this)}
# input
"not an error"
error("an error")
# expected output
{err:false,message:"not an error"}
{err:true,message:"an error"}
```
