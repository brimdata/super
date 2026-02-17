# upcast

convert a value to a supertype

## Synopsis

```
upcast(val: any, target: type) -> any
```

## Description

The `upcast` function is like [cast](cast.md) but does not perform any
type coercion and converts a value `val` from its type to any supertype of its type
as indicated by the `target` type argument.

When a record value does not contain a field in the super type, the value
`error("missing")` appears for that field, unless the field is nullable
(i.e., is a union type including type null), in which case the null value appears.

A type is a supertype of a subtype if all paths through the subtype are valid
paths through the supertype.

Upcasting is used by the [fuse](../../operators/fuse.md) operator.

When an upcast is successful, the return value of `cast` always has the target type.

If errors are encountered, then some or all of the resulting value
will be embedded with structured errors and the result does not have
the target type.

## Examples

---

_Upcast showing missing versus null_

```mdtest-spq {data-layout="stacked"}
# spq
values
  upcast({x:1},<{x:int64,y:string}>),
  upcast({x:1},<{x:int64,y:string|null}>)
# input

# expected output
{x:1,y:error("missing")}
{x:1,y:null::(string|null)}
```

---
