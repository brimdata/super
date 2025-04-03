### Function

&emsp; **unflatten** &mdash; transform an array of key/value records into a
record.

### Synopsis

```
unflatten(val: [{key:string|[string],value:any}]) -> record
```

### Description
The _unflatten_ function converts the key/value records in array `val` into
a single record. _unflatten_ is the inverse of _flatten_, i.e., `unflatten(flatten(r))`
will produce a record identical to `r`.

### Examples

Simple:
```mdtest-spq {data-layout="stacked"}
# spq
yield unflatten(this)
# input
[{key:"a",value:1},{key:["b"],value:2}]
# expected output
{a:1,b:2}
```

Flatten to unflatten:
```mdtest-spq
# spq
over flatten(this) => (
  key[1] != "rm"
  | yield collect(this)
)
| yield unflatten(this)
# input
{a:1,rm:2}
# expected output
{a:1}
```
