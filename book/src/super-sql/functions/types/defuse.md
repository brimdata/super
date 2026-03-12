# defuse

undo the effects of complete fusion

## Synopsis

```
defuse(val any) -> any
```

## Description

The `defuse` function converts a value `val` containing any fusion types
into its original type by downcasting all instances of fusion values to their
subtype equivalent.

## Examples

---

_Remove union types_

```mdtest-spq {data-layout="stacked"} runtime=sam
# spq
defuse(this)
# input
fusion({a:1::(int64|string)},<{a:int64}>)
fusion({a:"foo"::(int64|string)},<{a:string}>)
# expected output
{a:1}
{a:"foo"}
```

---

_Retain optional fields using complete fusion_

```mdtest-spq {data-layout="stacked"} runtime=sam
# spq
fuse | defuse(this)
# input
{x:1}
{x:2,y:3}
{x:4,z?:_::int64}
# expected output
{x:1}
{x:2,y:3}
{x:4,z?:_::int64}
```
