### Operator

&emsp; **shapes** &mdash; aggregate sample values by type

### Synopsis
```
shapes <expr>
```
### Description

The `shapes` operator aggregates its into by type and produces an
arbitrary sample value for each unique type in the input.

`shapes` is a shorthand for 
```
aggregate sample:=any(<expr>) by typeof(this) | values sample
```

> TODO: we should add an `is not null <expr>` as a filter


### Examples

---

```mdtest-spq
# spq
shapes | sort
# input
1
2
3
"foo"
"bar"
# expected output
1
"foo"
```
