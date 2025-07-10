### Maps

The map type conforms to the definition of the
[map type](../../formats/model.md#24-map)
in the super-structured data model.

Maps can be created by reading external data (SUP files,
database data, Parquet values, etc) or by
constructing instances using _map expressions_ or other
SuperSQL functions that produce maps.

Any SUP text defining an [map value](../../formats/sup.html#244-map-value)
is a valid map literal in the SuperSQL language.

#### Map Expressions

Map values are constructed from a _map expression_ that is comprised of
zero or more comma-separated key-value pairs contained in pipe braces:
```
|{ <key> : <value>, <key> : <value> ... }|
```
where an `<key>` and `<value> 
may be any valid [expression](../expressions.md).

> The map spread operator is not yet implemented.

When the expressions result in values of non-uniform type of either the keys or
the values, then their types become a sum type of the types present,
tied together with the corresponding [union type](union.md).

An empty map value has the form `{[]}`.

#### Map Type

A map type has the syntax defined for the
[map type](../../formats/sup.md#253-set-type)
in the [SUP format](../../formats/sup.md), i.e.,
```
{[ <key-type> : <value-type>]}
```
where `<key-type>` and `<value-type>` are any types.

An empty map type defaults to a map of key and value null type, i.e., `{[null:null]}`.

#### Examples

For example,
```mdtest-spq
# spq
values |{"foo":1,"bar"+"baz":2+3}|
# input
null
# expected output
|{"foo":1,"barbaz":5}|
```
