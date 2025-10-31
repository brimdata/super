### Function

&emsp; **cast** &mdash; convert a value to a different type

### Synopsis

```
cast(val: any, target: type) -> any
cast(val: any, name: string) -> any
```

### Description

The `cast` function implements a [cast](../../expressions/cast.md) where the target
of the cast is a [type value](../../types/type.md) instead of a type.

In the first form,
the function converts `val` to the type indicated by `target` in accordance 
with the semantics of the [expression cast](../../expressions/cast.md).

In the second form, the target type is a [named type](../../types/named.md)
whose name is the `name` parameter and whose type is the type of `val`.

When a cast is successful, the return value of `cast` always has the target type.

If errors are encountered, then some or all of the resulting value
will be embedded with structured errors and the result does not have
the target type.

### Examples

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

_Create a named type and cast value to the new type_

```mdtest-spq
# spq
cast(this, "foo")
# input
{a:1,b:2}
{a:3,b:4}
# expected output
{a:1,b:2}::=foo
{a:3,b:4}::=foo
```

---

_Derive type names from the properties of data_

```mdtest-spq
# spq
switch
  case has(x) ( cast(this, "point") )
  default ( cast(this, "radius") )
| sort this
# input
{x:1,y:2}
{r:3}
{x:4,y:5}
# expected output
{r:3}::=radius
{x:1,y:2}::=point
{x:4,y:5}::=point
```
