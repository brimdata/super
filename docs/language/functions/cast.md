### Function

&emsp; **cast** &mdash; coerce a value to a different type

### Synopsis

```
cast(val: any, t: type) -> any
cast(val: any, name: string) -> any
```

### Description

The _cast_ function performs type casts but handles both [primitive types](../../formats/zed.md#1-primitive-types) and
[complex types](../../formats/zed.md#2-complex-types).  If the input type `t` is a primitive type, then the result
is equivalent to
```
t(val)
```
e.g., the result of `cast(1, <string>)` is the same as `string(1)` which is `"1"`.
In the second form, where the `name` argument is a string, cast creates
a new [named type](../data-types.md#named-types) where the name for the type is given by `name` and its
type is given by `typeof(val)`.  This provides a convenient mechanism
to create new named types from the input data itself without having to
hard code the type in the SuperPipe query.

For complex types, the cast function visits each leaf value in `val` and
casts that value to the corresponding type in `t`.
When a complex value has multiple levels of nesting,
casting is applied recursively down the tree.  For example, cast is recursively
applied to each element in an array of records and recursively applied to each record.

If `val` is a record (or if any of its nested value is a record):
* absent fields are ignored and omitted from the result,
* extra input fields are passed through unmodified to the result, and
* fields are matched by name and are order independent and the _input_ order is retained.

In other words, `cast` does not rearrange the order of fields in the input
to match the output type's order but rather just modifies the leaf values.

If a cast fails, an error is returned when casting to primitive types
and the input value is returned when casting to complex types.

{{% tip "Note" %}}

Many users seeking to `cast` record values prefer to use the
[`shape` function](./shape.md) which applies the `cast`, [`fill`](./fill.md),
and [`order`](./order.md) functions simultaneously.

{{% /tip %}}

### Examples

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

_Cast a record to a different record type_
```mdtest-spq
# spq
cast(this, <{b:string}>)
# input
{a:1,b:2}
{a:3}
{b:4}
# expected output
{a:1,b:"2"}
{a:3}
{b:"4"}
```

_Create a named type and cast value to the new type_
```mdtest-spq
# spq
cast(this, "foo")
# input
{a:1,b:2}
{a:3,b:4}
# expected output
{a:1,b:2}(=foo)
{a:3,b:4}(=foo)
```

_Derive type names from the properties of data_
```mdtest-spq
# spq
switch (
  case has(x) => cast(this, "point")
  default => cast(this, "radius")
)
| sort this
# input
{x:1,y:2}
{r:3}
{x:4,y:5}
# expected output
{r:3}(=radius)
{x:1,y:2}(=point)
{x:4,y:5}(=point)
```
