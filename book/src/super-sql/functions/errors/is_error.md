### Function

&emsp; **is_error** &mdash; test if a value is an error

### Synopsis

```
is_error(val: any) -> bool
```

### Description

The `is_error` function returns true if its argument's type is an error.
`is_error(v)` is shorthand for `kind(v)=="error"`,

### Examples

---

_A simple value is not an error_

```mdtest-spq
# spq
values is_error(this)
# input
1
# expected output
false
```

---

_An error value is an error_

```mdtest-spq
# spq
values is_error(this)
# input
error(1)
# expected output
true
```

---

_Convert an error string into a record with an indicator and a message_

```mdtest-spq
# spq
values {err:is_error(this),message:under(this)}
# input
"not an error"
error("an error")
# expected output
{err:false,message:"not an error"}
{err:true,message:"an error"}
```
