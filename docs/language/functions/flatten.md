### Function

&emsp; **flatten** &mdash; transform a record into a flattened array.

### Synopsis

```
flatten(val: record) -> [{key:[string],value:<any>}]
```

### Description
The _flatten_ function returns an array of records `[{key:[string],value:<any>}]`
where `key` is a string array of the path of each record field of `val` and
`value` is the corresponding value of that field.
If there are multiple types for the leaf values in `val`, then the array value
inner type is a union of the record types present.

### Examples

```mdtest-spq {data-layout="stacked"}
# spq
values flatten(this)
# input
{a:1,b:{c:"foo"}}
# expected output
[{key:["a"],value:1},{key:["b","c"],value:"foo"}]
```
