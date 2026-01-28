# trim

strip leading and trailing whitespace

## Synopsis

```
trim(s: string) -> string
```

## Description

The `trim` function strips all leading and trailing whitespace
from string argument `s` and returns the result.

## Examples

---

```mdtest-spq
# spq
values trim(this)
# input
" = SuperDB = "
# expected output
"= SuperDB ="
```
