### Records

The record type follows the definition of the
[record type](../../formats/model.md#21-record) from the 
super-structured data model.

Records can be created by reading external data (SUP files, 
database data, Parquet values, JSON objects, etc) or by 
constructing instances using
[record expressions](../expressions.html#record-expressions) or other 
SuperSQL functions that produce records.

Any SUP text defining a [record value](../../formats/sup.md#241-record-value)
is a valid record literal in the SuperSQL language.

For example, this record
```
{b:true,u:1::uint8,a:[1,2,3],s:"hello"::=CustomString}
```
is a valid serialized SUP record and is also a valid SuperSQL expression, e.g.,
```mdtest-spq
# spq
values {b:true,u:1::uint8,a:[1,2,3],s:"hello"::=CustomString}
# input
null
# expected output
{b:true,u:1::uint8,a:[1,2,3],s:"hello"::=CustomString}
```

#### Record Expressions

Record values are constructed from a _record expression_ that is comprised of
zero or more comma-separated elements contained in braces:
```
{ <element>, <element>, ... }
```
where an `<element>` has one of three forms:

* a named field of the form `<name> : <expr>`  where `<name>` is an
[identifier](xxx) or 
[string](xxx)
and `<expr>` is an arbitrary [expression](../expressions.md),
* a single [field reference]() in the form `<id>` of an 
[identifier](xxx), which is shorthanf for the named field reference `<id>:<id>`, or
* a spread expression of the form `...<expr>` where `<expr>` is an arbitrary 
[expression](../expressions.md) that should evaluate to a record value.
```

The spread form inserts all of the fields from the resulting record.
If a spread expression results in a non-record type (e.g., errors), then that
part of the record is simply elided.

The fields of a record expression are evaluated left to right and when
field names collide the rightmost instance of the name determines that
field's value.

An empty record value has the form `{}`.

#### Record Type

A record type has the syntax defined for the
[record type](../../formats/sup.md#251-record-type)
in the [SUP format](../../formats/sup/md), i.e.,
```
{ <name> : <type>, <name> : <type>, ... }
```
where `<name>` is an
[identifier](xxx) or 
[string](xxx)
as in record expressions and `<type>` is any type.

An empty record type has the same form as an empty record value, i.e., `{}`.

#### Examples

For example,
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

