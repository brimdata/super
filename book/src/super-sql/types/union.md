### Unions

The union data type conforms to the definition of the
[union type](../../formats/model.md#21-union) in the 
super-structured data model.

Union values can be created by reading external data (SUP files,
database data, JSON objects, etc),
by constructing instances with a [type cast](../expressions.md#casts)
or with other SuperSQL functions or expressions that produce unions.

In particular, array, set, and map expressions all produce union types
when they are comprised of mix-type elements.

#### Union Type

A union type has the syntax defined for the
[union type](../../formats/sup.md#251-union-type)
in the [SUP format](../../formats/sup/md), i.e.,
```
<type> | <type> | ...
```
where `<type>` is any type.

#### Union Value Semantics

TODO 

add type discriminator syntax, e.g., `u.(uint64)` ?

#### Algebraic Types

TODO

#### Examples

_A union composed of the types `int64` and `string`
is expressed as `int64|string` and any value that has a type
that appears in the union type may be cast to that union type.
Since 1 is an `int64` and "foo" is a `string`, they both can be
values of type `int64|string`._
```mdtest-spq
# spq
values this::(<int64|string>)
# input
1
"foo"
# expected output
1::(int64|string)
"foo"::(int64|string)
```

_The value underlying a union-tagged value is accessed with the
[`under` function](../functions/under.md)._
```mdtest-spq
# spq
values under(this)
# input
1::(int64|string)
# expected output
1
```

---

Union values are powerful because they provide a mechanism to precisely
describe the type of any nested, semi-structured value composed of elements
of different types.  For example, the type of the value `[1,"foo"]` in JavaScript
is simply a generic JavaScript "object".  But in SuperSQL, the type of this
value is an array of union of string and integer, e.g.,
```mdtest-spq
# spq
typeof(this)
# input
[1,"foo"]
# expected output
<[int64|string]>
```
