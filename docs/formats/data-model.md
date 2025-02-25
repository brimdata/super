---
weight: 1
title: Data Model
---

Super-structured data is defined as an ordered sequence of one or more typed data values.
Each value's type is either a "primitive type", a "complex type", the "type type",
a "named type", or the "null type".

## 1. Primitive Types

Primitive types include signed and unsigned integers, IEEE binary and decimal
floating point, string, byte sequence, Boolean, IP address, IP network,
null, and a first-class type _type_.

There are 30 types of primitive values with syntax defined as follows:

| Name       | Definition                                      |
|------------|-------------------------------------------------|
| `uint8`    | unsigned 8-bit integer  |
| `uint16`   | unsigned 16-bit integer |
| `uint32`   | unsigned 32-bit integer |
| `uint64`   | unsigned 64-bit integer |
| `uint128`  | unsigned 128-bit integer |
| `uint256`  | unsigned 256-bit integer |
| `int8`     | signed 8-bit integer    |
| `int16`    | signed 16-bit integer   |
| `int32`    | signed 32-bit integer   |
| `int64`    | signed 64-bit integer   |
| `int128`   | signed 128-bit integer   |
| `int256`   | signed 256-bit integer   |
| `duration` | signed 64-bit integer as nanoseconds |
| `time`     | signed 64-bit integer as nanoseconds from epoch |
| `float16`  | IEEE-754 binary16 |
| `float32`  | IEEE-754 binary32 |
| `float64`  | IEEE-754 binary64 |
| `float128`  | IEEE-754 binary128 |
| `float256`  | IEEE-754 binary256 |
| `decimal32`  | IEEE-754 decimal32 |
| `decimal64`  | IEEE-754 decimal64 |
| `decimal128`  | IEEE-754 decimal128 |
| `decimal256`  | IEEE-754 decimal256 |
| `bool`     | the Boolean value `true` or `false` |
| `bytes`    | a bounded sequence of 8-bit bytes |
| `string`   | a UTF-8 string |
| `ip`       | an IPv4 or IPv6 address |
| `net`      | an IPv4 or IPv6 address and net mask |
| `type`     | a type value |
| `null`     | the null type |

The _type_ type  provides for first-class types. Even though a type value can
represent a complex type, the value itself is a singleton.

Two type values are equivalent if their underlying types are equal.  Since
every type in the type system is uniquely defined, type values are equal
if and only if their corresponding types are uniquely equal.

The _null_ type is a primitive type representing only a `null` value.
A `null` value can have any type.

{{% tip "Note" %}}

`time` values correspond to 64-bit epoch nanoseconds and thus
not every valid RFC 3339 date and time string represents a valid time.
In addition, nanosecond epoch times overflow on April 11, 2262.
For the world of 2262, a new epoch can be created well in advance
and the old time epoch and new time epoch can live side by side with
the old using a [named type](#3-named-type) for the new epoch time referring to the old `time`.
An app that wants more than 64 bits of timestamp precision can always use
a named type of a `bytes` type and do its own conversions to and from the
corresponding bytes values.  A time with a local time zone can be represented
as a record of a `time` field and a zone field.

{{% /tip %}}

## 2. Complex Types

Complex types are composed of primitive types and/or other complex types.
The categories of complex types include:
* _record_ - an ordered collection of zero or more named values called fields,
* _array_ - an ordered sequence of zero or more values called elements,
* _set_ - a set of zero or more unique values called elements,
* _map_ - a collection of zero or more key/value pairs where the keys are of a
uniform type called the key type and the values are of a uniform type called
the value type,
* _union_ - a type representing values whose type is any of a specified collection of two or more unique types,
* _enum_ - a type representing a finite set of symbols typically representing categories, and
* _error_ - any value wrapped as an "error".

The type system comprises a total order:
* The order of primitive types corresponds to the order in the table above.
* All primitive types are ordered before any complex types.
* The order of complex type categories corresponds to the order above.
* For complex types of the same category, the order is defined below.

### 2.1 Record

A record comprises an ordered set of zero or more named values
called "fields".  The field names must be unique in a given record
and the order of the fields is significant, e.g., type `{a:string,b:string}`
is distinct from type `{b:string,a:string}`.

A field name is any UTF-8 string.

A field value is a value of any type.

In contrast to many schema-oriented data formats, the super data model has no way to specify
a field as "optional" since any field value can be a null value.

If an instance of a record value omits a value
by dropping the field altogether rather than using a null, then that record
value corresponds to a different record type that elides the field in question.

A record type is uniquely defined by its ordered list of field-type pairs.

The type order of two records is as follows:
* Record with fewer columns than other is ordered before the other.
* Records with the same number of columns are ordered as follows according to:
     * the lexicographic order of the field names from left to right,
     * or if all the field names are the same, the type order of the field types from left to right.

### 2.2 Array

An array is an ordered sequence of zero or more values called "elements"
all conforming to the same type.

An array value may be empty.  An empty array may have element type `null`.

An array type is uniquely defined by its single element type.

The type order of two arrays is defined as the type order of the
two array element types.

An array of mixed-type values (such a mixed-type JSON array) is representable
as an array with elements of type `union`.

### 2.3 Set

A set is an unordered sequence of zero or more values called "elements"
all conforming to the same type.

A set may be empty.  An empty set may have element type `null`.

A set of mixed-type values is representable as a set with
elements of type `union`.

A set type is uniquely defined by its single element type.

The type order of two sets is defined as the type order of the
two set element types.

### 2.4 Map

A map represents a list of zero or more key-value pairs, where the keys
have a common type and the values have a common type.

Each key across an instance of a map value must be a unique value.

A map value may be empty.

A map type is uniquely defined by its key type and value type.

The type order of two map types is as follows:
* the type order of their key types,
* or if they are the same, then the order of their value types.

### 2.5 Union

A union represents a value that may be any one of a specific enumeration
of two or more unique data types that comprise its "union type".

A union type is uniquely defined by an ordered set of unique types (which may be
other union types) where the order corresponds to the type system's total order.

Union values are tagged in that
any instance of a union value explicitly conforms to exactly one of the union's types.
The union tag is an integer indicating the position of its type in the union
type's ordered list of types.

The type order of two union types is as follows:
* The union type with fewer types than other is ordered before the other.
* Two union types with the same number of types are ordered according to
the type order of the constituent types in left to right order.

### 2.6 Enum

An enum represents a symbol from a finite set of one or more unique symbols
referenced by name.  An enum name may be any UTF-8 string.

An enum type is uniquely defined by its ordered set of unique symbols,
where the order is significant, e.g., two enum types
with the same set of symbols but in different order are distinct.

The type order of two enum types is as follows:
* The enum type with fewer symbols than other is ordered before the other.
* Two enum types with the same number of symbols are ordered according to
the type order of the constituent types in left to right order.

### 2.7 Error

An error represents any value designated as an error.

The type order of an error is the type order of the type of its contained value.

## 3. Named Type

A _named type_ is a name for a specific data type.
Any value can have a named type and the named type is a distinct type
from the underlying type.  A named type can refer to another named type.

The binding between a named type and its underlying type is local in scope
and need not be unique across a sequence of values.

A type name may be any UTF-8 string exclusive of primitive type names.

For example, if "port" is a named type for `uint16`, then two values of
type "port" have the same type but a value of type "port" and a value of type `uint16`
do not have the same type.

The type order of a named type is the type order of its underlying type with two
exceptions:
* A named type is ordered after its underlying type.
* Named types sharing an underlying type are ordered lexicographically by name.

{{% tip "Note" %}}

While the data model does not include explicit support for schema versioning,
named types provide a flexible mechanism to implement versioning
on top of the super-structured serialization formats.  For example, a system
could define a naming convention of the form `<type>.<version>`
where `<type>` is the type name of a record representing the schema
and `<version>` is a decimal string indicating the version of that schema.
Since types need only be parsed once per stream
in the serialization formats, a super-structured type implementation could
efficiently support schema versioning using such a convention.

{{% /tip %}}

## 4. Null Values

All data types have a null representation.  It is up to an
implementation to decide how external data structures map into and
out of values with nulls.  Typically, a null value is either the
zero value or, in the case of record fields, an optional field whose
value is not present, though these semantics are not explicitly
defined by the super data model.
