### Function

&emsp; **has_error** &mdash; test if a value is or contains an error

### Synopsis

```
has_error(val: any) -> bool
```

### Description

The _has_error_ function returns true if its argument is or contains an error.
_has_error_ is different from _is_error_ in that _has_error_ will recurse
into value's leaves to determine if there is an error in the value.

### Examples

```mdtest-spq
# spq
yield has_error(this)
# input
{a:{b:"foo"}}
# expected output
false
```

```mdtest-spq
# spq
a.x := a.y + 1 | yield has_error(this)
# input
{a:{b:"foo"}}
# expected output
true
```
