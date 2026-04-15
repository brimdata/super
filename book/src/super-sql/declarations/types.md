# Types

Named types are declared with the syntax
```
type <id> = <type>
```
where `<id>` is an [identifier](../queries.md#identifiers) and `<type>` is a [type](../types/intro.md).
This creates a new type with the given name in the type system.

Type declarations must appear in the declaration section of a [scope](../queries.md#scope).

Any named type that appears in the body of a type declaration must be previously
declared in the same scope or in an ancestor scope, i.e., types cannot contain
forward references to other named types.  In particular, named types cannot be recursive.

>[!NOTE]
> A future version of SuperSQL may include recursive types.  This is a research topic
> for the SuperDB project.

Input data may create [named types](../../formats/model.md#3-named-type) that conflict with type declarations, which causes an error.

Types can also be bound to identifiers without creating a named type using a
[constant](constants.md) declaration binding the name to a [type value](../types/type.md).

## Examples

---

_Cast integers to a network port type_

```mdtest-spq
# spq
type port=uint16
values this::port
# input
80
# expected output
80::(port=uint16)
```

---

_Cast integers to a network port type calling `cast` with a type value_

```mdtest-spq
# spq
type port=uint16
values cast(this, <port>)
# input
80
# expected output
80::(port=uint16)
```

---

_Override binding to type name with `this`_

```mdtest-spq
# spq
type foo=string
values cast(x, foo), cast(x, this.foo)
# input
{x:1,foo:<float64>}
{x:2,foo:<bool>}
# expected output
"1"::=foo
1.
"2"::=foo
true
```

---

_A type name argument to `cast` in the form of a string is independent of type declarations_

```mdtest-spq
# spq
type foo=int64
values {str:cast(this, 'foo'), named:cast(this, foo)}
# input
1
2
# expected output
{str:1::=foo,named:1::foo}
{str:2::=foo,named:2::foo}
```

---

_Bind a name to a type without creating a named type_

```mdtest-spq
# spq
const foo=<string>
values this::foo
# input
1
2
# expected output
"1"
"2"
```