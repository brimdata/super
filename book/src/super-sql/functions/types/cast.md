# cast

convert a value to a different type

## Synopsis

```
cast(val: any, target: type) -> any
```

## Description

The `cast` function implements a [cast](../../expressions/cast.md) where the target
of the cast is a [type value](../../types/type.md) instead of a type.

The function converts `val` to the type indicated by `target` in accordance
with the semantics of the [expression cast](../../expressions/cast.md).

When a cast is successful, the return value of `cast` always has the target type.

If errors are encountered, then some or all of the resulting value
will be embedded with structured errors and the result does not have
the target type.

## Examples

---

_Cast primitives to type `ip`_

```mdtest-spq {data-layout="stacked"}
# spq
cast(this, <ip>)
# input
"10.0.0.1"
1
"foo"
# expected output
10.0.0.1
error({message:"cannot cast to ip",on:1})
error({message:"cannot cast to ip",on:"foo"})
```

---

_Cast a record to a different record type_

```mdtest-spq
# spq
cast(this, <{b:string}>)
# input
{a:1,b:2}
{a:3}
{b:4}
# expected output
{b:"2"}
{b:null::string}
{b:"4"}
```

---

_Cast using a computed type value_

```mdtest-spq
# spq
values cast(val, type)
# input
{val:"123",type:<int64>}
{val:"123",type:<float64>}
{val:["true","false"],type:<[bool]>}
# expected output
123
123.
[true,false]
```
