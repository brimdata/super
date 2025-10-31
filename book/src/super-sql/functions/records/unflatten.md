### Function

&emsp; **unflatten** &mdash; transform an array of key/value records into a
record

### Synopsis

```
unflatten(val: [{key:string|[string],value:any}]) -> record
```

### Description

The `unflatten` function converts the key/value records in array `val` into
a single record. _unflatten_ is the inverse of _flatten_, i.e., `unflatten(flatten(r))`
will produce a record identical to `r`.

### Examples

---

_Unflatten a single simple record_

```mdtest-spq {data-layout="stacked"}
# spq
values unflatten(this)
# input
[{key:"a",value:1},{key:["b"],value:2}]
# expected output
{a:1,b:2}
```

---

_Flatten to unflatten_

```mdtest-spq
# spq
unnest flatten(this) into (
  key[0] != "rm"
  | values collect(this)
)
| values unflatten(this)
# input
{a:1,rm:2}
# expected output
{a:1}
```
