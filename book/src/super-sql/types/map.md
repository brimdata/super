### Maps

Maps conform to the
[map type](../../formats/model.md#24-map)
in the super-structured data model and follow the
[syntax](../../formats/sup.md#254-map-type)
of maps in the [SUP format](../../formats/sup.md), i.e.,
a map type has the form
```
|{ <key> : <value> }|
```
where `<key>` and `<value>` are any types and represent the keys
and values types of the map.

Any SUP text defining a [map value](../../formats/sup.md#244-map-value)
is a valid map literal in the SuperSQL language.

For example, this is a simple map value
```
|{"foo":1,"bar":2}|
```
whose type is
```
|{string:int64}|
```

An empty map value has the form `|{}|` and
an empty map type defaults to a map with null types, i.e., `|{null:null}|`,
unless otherwise cast, e.g., `|{}|::|{string:int64}|` represents an empty
map of string keys and integer values.

Maps can be created by reading external data (SUP files,
database data, Parquet values, etc) or by
constructing instances using [_map expressions_](#map-expressions) or other
SuperSQL functions that produce maps.

#### Map Expressions

Map values are constructed from a _map expression_ that is comprised of
zero or more comma-separated key-value pairs contained in pipe braces:
```
|{ <key> : <value>, <key> : <value> ... }|
```
where `<key>` and `<value>`
may be any valid [expression](../expressions.md).

> _The map spread operator is not yet implemented._

When the expressions result in values of non-uniform type of either the keys or
the values, then their types become a sum type of the types present,
tied together with the corresponding [union type](union.md).

#### Examples
---
```mdtest-spq
# spq
values |{"foo":1,"bar"+"baz":2+3}|
# input
null
# expected output
|{"foo":1,"barbaz":5}|
```
---
_Look up network of host in a map and annotate if present_
```mdtest-spq {data-layout="stacked"}
# spq
const networks = |{
    192.168.1.0/24:"private net 1",
    192.168.2.0/24:"private net 2",
    10.0.0.0/8:"net 10"
}|
note:=coalesce(networks[network_of(host)], f"unknown network for host {host}")
# input
{host:192.168.1.100}
{host:192.168.2.101}
{host:192.168.3.102}
{host:10.0.0.1}
# expected output
{host:192.168.1.100,note:"private net 1"}
{host:192.168.2.101,note:"private net 2"}
{host:192.168.3.102,note:"unknown network for host 192.168.3.102"}
{host:10.0.0.1,note:"net 10"}
```
