# has_error

test if a value is or contains an error

## Synopsis

```
has_error(val: any) -> bool
```

## Description

The `has_error` function returns true if its argument is or contains an error.
`has_error` is different from
[`is_error`](is_error.md) in that `has_error` recursively
searches a value to determine if there is any error in a nested value.

## Examples

---

```mdtest-spq
# spq
values has_error(this)
# input
{a:{b:"foo"}}
# expected output
false
```

---

```mdtest-spq
# spq
a.x := a.y + 1 | values has_error(this)
# input
{a:{b:"foo"}}
# expected output
true
```
