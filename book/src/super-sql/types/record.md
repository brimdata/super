### Records

Records conform to the
[record type](../../formats/model.md#21-record) in the
super-structured data model and follow the
[syntax](../../formats/sup.md#251-record-type)
of records in the [SUP format](../../formats/sup.md), i.e.,
a record type has the form
```
{ <name> : <type>, <name> : <type>, ... }
```
where `<name>` is an identifier or string
and `<type>` is any type.

Any SUP text defining a [record value](../../formats/sup.md#241-record-value)
is a valid record literal in the SuperSQL language.

For example, this is a simple record value
```
{number:1,message:"hello,world"}
```
whose type is
```
{number:int64,message:string}
```
An empty record value and an empty record type are both represented as `{}`.

Records can be created by reading external data (SUP files,
database data, Parquet values, JSON objects, etc) or by
constructing instances using
[record expressions](#record-expressions) or other
SuperSQL functions that produce records.

#### Record Expressions

Record values are constructed from a _record expression_ that is comprised of
zero or more comma-separated elements contained in braces:
```
{ <element>, <element>, ... }
```
where an `<element>` has one of three forms:

* a named field of the form `<name> : <expr>`  where `<name>` is an
identifier or string and `<expr>` is an arbitrary [expression](../expressions.md),
* a single field reference in the form `<id>` as an identifier,
which is shorthand for the named field reference `<id>:<id>`, or
* a spread expression of the form `...<expr>` where `<expr>` is an arbitrary
[expression](../expressions.md) that should evaluate to a record value.

The spread form inserts all of the fields from the resulting record.
If a spread expression results in a non-record type (e.g., errors), then that
part of the record is simply elided.

The fields of a record expression are evaluated left to right and when
field names collide the rightmost instance of the name determines that
field's value.

#### Examples
---
```mdtest-spq
# spq
values {a:0},{x}, {...r}, {a:0,...r,b:3}
# input
{x:1,y:2,r:{a:1,b:2}}
# expected output
{a:0}
{x:1}
{a:1,b:2}
{a:1,b:3}
```
---
```mdtest-spq {data-layout="stacked"}
# spq
values {b:true,u:1::uint8,a:[1,2,3],s:"hello"::=CustomString}
# input
null
# expected output
{b:true,u:1::uint8,a:[1,2,3],s:"hello"::=CustomString}
```
