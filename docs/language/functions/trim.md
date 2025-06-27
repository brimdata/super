### Function

&emsp; **trim** &mdash; strip leading and trailing whitespace

### Synopsis

```
trim(s: string) -> string
```

### Description

The _trim_ function converts stips all leading and trailing whitespace
from string argument `s` and returns the result.

### Examples

```mdtest-spq
# spq
values trim(this)
# input
" = SuperDB = "
# expected output
"= SuperDB ="
```
