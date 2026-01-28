# Named Types

A named type provides a means to bind a symbolic name to a type
and conforms to the
[named type](../../formats/model.md#3-named-type)
in the super-structured data model.
The named type [syntax](../../formats/sup.md#258-named-type)
follows that of [SUP format](../../formats/sup.md), i.e.,
a named type has the form
```
(<name>=<type>)
```
where `<name>` is an identifier or string and `<type>` is any type.

Named types may be defined in four ways:
* with a [type](../declarations/types.md) declaration,
* with a [cast](../expressions/cast.md),
* with a definition inside of another type, or
* by the input data itself.

For example, this expression
```
80::(port=uint16)
```
casts the integer 80 to a named type called `port` whose type is `uint16`.

Alternatively, named types can be declared with a type statement, e.g.,
```
type port = int16
values 80::port
```
produces the value `80::(port=uint16)` as above.

Type name definitions can be embedded in another type, e.g.,
```
type socket = {addr:ip,port:(port=uint16)}
```
defines a named type `socket` that is a record with field `addr` of type `ip`
and field `port` of type `port`, where type `port` is a named type for type `uint16` .

Named types may also be defined by the input data itself, as super-structured data is
comprehensively self describing.
When named types are defined in the input data, there is no need to declare their
type in a query.
In this case, a SuperSQL expression may refer to the type by the name that simply
appears to the runtime as a side effect of operating upon the data.

When the same name is bound to different types, a reference to that name is
undefined except for the definitions within a single nested value,
in which case, the most recent binding in depth-first order is used to resolve
a reference to a type name.

## Examples

---

_Filter on a type name defined in the input data_

```mdtest-spq
# spq
where typeof(this)==<foo>
# input
1::=foo
2::=bar
3::=foo
# expected output
1::=foo
3::=foo
```

---

_Emit a type name defined in the input data_

```mdtest-spq
# spq
values <foo>
# input
1::=foo
# expected output
<foo=int64>
```

---

_Emit a missing value for an unknown type name_

```mdtest-spq
# spq
values <foo>
# input
1
# expected output
error("missing")
```

---

_Conflicting named types appear as distinct type values_

```mdtest-spq {data-layout="stacked"}
# spq
count() by typeof(this) | sort this
# input
1::=foo
2::=bar
"hello"::=foo
3::=foo
# expected output
{typeof:<bar=int64>,count:1}
{typeof:<foo=int64>,count:2}
{typeof:<foo=string>,count:1}
```
