### Function

&emsp; **cast** &mdash; convert a value to a different type

### Synopsis

```
cast(val: any, target: type) -> any
cast(val: any, name: string) -> any
```

### Description

The `cast` function converts a value in one type (the type of `val`) to another
as indicated by the target type.  In the first form, the target type is the
[type value](../../types/type.md) specified by `target`.

In the second form, the target type is a [named type](../../types/named.md)
whose name is the `name` parameter and whose type is the type of `val`.

When a cast is successful, the return value of `cast` always has the target type.

If errors are encountered, then some or all of the resulting value
will be embedded with structured errors and the result does not have
the target type.

The target type cannot contain an error type.  The [`error`](../errors/error.md) function
should instead be used to create error values.

SQL syntax for casting, e.g., `CAST(<expr> AS <type>)` and `<expr>::<type>`,
is also [supported](../../expressions.md#casts).  These forms are
internally converted to calls to [`cast`](#function).

### Primitive Values

Some primitive values can be cast to other primitive types, but not all possibilities are
permitted and instead result in structured errors.
Except for [union](../../types/union.md) and
[named](../../types/named.md) types,
primitive values cannot be cast to complex types.

The casting rules for primitives are as follows:
* A [number](../../types/numbers.md) may be cast to
  * another [number](../../types/numbers.md) type as long as the numeric value
    is not outside the scope of the target type,
    which results in a structured error,
  * type [`string`](../../types/string.md),
  * type [`bool`](../../types/bool.md) where zero is `false` and non-zero is `true`,
  * type [`duration`](../../types/time.md) where the number is presumed to be nanoseconds,
  * type [`time`](../../types/time.md) where the number is presumed to be nanoseconds since epoch, or
  * a [union](#union-types) or [named type](#named-types).
* A [string](../../types/string.md) may be cast to any other primitive type as long as
the string corresponds to a valid SuperSQL primitive literal.  Time strings
in particular may represent typical timestamp formats.  When cast to the
[`bytes`](../../types/bytes.md) type,
the result is the byte encoding of the UTF-8 string.  A string may also be cast to
a [union](#union-types) or [named](#named-types) type.
To parse a literal
string that is in the SUP or JSON format without having to specify the target type, use
the [`parse_sup`](../parsing/parse_sup.md) function.
* A [bool](../../types/bool.md) may be cast to
  * a number type where `false` is zero and `true` is `1`,
  * type [`string`](../../types/string.md), or
  * a [union](#union-types) or [named type](#named-types).
* A [time](../../types/time.md) value may be cast to
  * a number type where the numeric value is nanoseconds since epoch
  * type [`string`](../../types/string.md), or
  * a [union](#union-types) or [named type](#named-types).

A null value of type [null](../../types/null.md) may be cast to any type.

> _A future version of this documentation will provide detailed documentation for
> acceptable date/time strings._

### Complex Values

When a complex value has multiple levels of nesting,
casting is applied recursively into the value hierarchy.
For example, cast is recursively
applied to each element in an array of records, then recursively applied to each
of those records.

If there is a mismatch between the type of the input value and target type then
structured errors appear within the portion of a nested value that
is not castable.

The casting rules for complex values are as follows:

* A [record](../../types/record.md) may be cast to
  * a [record](../../types/record.md) type where any fields not present in the
    target type are omitted, any fields not present in the input value while present in the
    target type are set to null, and the value of each input field present
    in both the input and target are recursively cast to the target's type of
    that field,
  * a [string](../../types/string.md) type where the string is the input value
    serialized in the [SUP](../../../formats/sup.md) format, or
  * a [union](#union-types) or [named type](#named-types).
* An [array](../../types/array.md) may be cast to
  * an [array](../../types/array.md) type where the elements of the input value are
    recursively cast to the element type of the target array type,
  * a [set](../../types/set.md) type where the elements of the input value are
    recursively cast to the element type of the target set type and any duplicate
    values are automatically removed, or
  * a [string](../../types/string.md) type where the string is the input value
    serialized in the [SUP](../../../formats/sup.md) format, or
  * a [union](#union-types) or [named type](#named-types).
* A [set](../../types/set.md) may be cast to
  * a [set](../../types/set.md) type where the elements of the input value are
    recursively cast to the element type of the target set type,
  * an [array](../../types/array.md) type where the elements of the input value are
    recursively cast to the element type of the target array type, or
  * a [string](../../types/string.md) type where the string is the input value
    serialized in the [SUP](../../../formats/sup.md) format, or
  * a [union](#union-types) or [named type](#named-types).
* A [map](../../types/map.md) may be cast to
  * a [map](../../types/map.md) type where the keys and values of the input value are
    recursively cast to the key and value type of the target map type, or
  * a [string](../../types/string.md) type where the string is the input value
    serialized in the [SUP](../../../formats/sup.md) format, or
  * a [union](#union-types) or [named type](#named-types).
* An [enum](../../types/enum.md) may be cast to
  * an [enum](../../types/enum.md) type where the target type includes the symbol
    of the value being cast, or
  * a [string](../../types/string.md) type where the string is the input value
    serialized in the [SUP](../../../formats/sup.md) format, or
  * a [union](#union-types) or [named type](#named-types).

### Union Types

When casting a value to a union type, the member type of the union is selected
to find a _best fit_ of the available types.  If no fit exists, a structured
error is returned.

If the input type is present in the member types, then the best fit is that type.

Otherwise, the best fit is determined from the input type as follows:

> _A future version of this documentation will provide detailed documentation for
> best-fit selection algorithm._

### Named Types

When casting to a named type, the cast is carried out using its underlying type
then the named type is reattached to the result.

### Errors

Casts attempted between a value and a type that are not defined
result in a structured error of the form of:
```
{message:"cannot cast to <target>", on:<val>}
```
When errors appear within a complex value, the returned
value may not be wrapped in a structured error and the problematic portions
of the cast can be debugged by inspecting the result for precisely where
the errors arose.

For example, this function call
```
cast({a:"1",b:2}, <{a:int64,b:ip}>)
```
returns
```
{a:1,b:error({message:"cannot cast to ip",on:2})}
```
That is the value for `a` was successfully cast from string `"1`" to integer `1` but
the value for `b` could not be cast to an IP address so a structured error is
instead embedded as the value for `b`.

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

_Multiple syntax options for casting_

```mdtest-spq
# spq
values
  cast(80::uint16, 'port'),
  cast(cast(80, <uint16>), 'port'),
  CAST(80 AS (port=uint16)),
  80::(port=uint16)
# input
null
# expected output
80::(port=uint16)
80::(port=uint16)
80::(port=uint16)
80::(port=uint16)
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
