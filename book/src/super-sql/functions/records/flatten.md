# flatten

transform a record into a flattened array

## Synopsis

```
flatten(val: record) -> [{key:[string],value:<any>}]
```

## Description

The `flatten` function returns an array of records `[{key:[string],value:<any>}]`
where `key` is a string array of the path of each record field of `val` and
`value` is the corresponding value of that field.
If there are multiple types for the leaf values in `val`, then the array value
inner type is a union of the record types present.

>[!NOTE]
> A future version of `flatten` will support all nested data types (e.g., maps, sets, etc)
> where the array-of-strings value of key becomes a more general data structure representing
> all possible value types that comprise a path.

## Examples

---

```mdtest-spq {data-layout="stacked"}
# spq
values flatten(this)
# input
{a:1,b:{c:"foo"}}
# expected output
[{key:["a"],value:1},{key:["b","c"],value:"foo"}]
```
