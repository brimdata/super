### Function

&emsp; **position** &mdash; find position of a substring

### Synopsis

```
position(s: string, sub: string) -> int64
position(sub: string IN s:string) -> int64
```

### Description

The _position_ function returns the 1-based index where string `sub` first
occurs in string `s`. If `sub` is not a sub-string of `s` then 0 is returned.

### Examples

```mdtest-spq
# spq
yield position(s, sub)
# input
{s:"foobar",sub:"bar"}
# expected output
4
```
