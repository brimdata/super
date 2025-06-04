### Function

&emsp; **typename** &mdash; look up and return a named type

### Synopsis

```
typename(name: string) -> type
```

### Description

The `typename` function returns the [type](../../types/intro.md) of the
[named type](../../types/named.md) given by `name` if it exists.  Otherwise,
`error("missing")` is returned.

### Examples

---

_Return a simple named type with a string constant argument_

```mdtest-spq
# spq
values typename("port")
# input
80::(port=int16)
# expected output
<port=int16>
```

---

_Return a named type using an expression_

```mdtest-spq
# spq
values typename(name)
# input
{name:"port",p:80::(port=int16)}
# expected output
<port=int16>
```

---

_The result is `error("missing")` if the type name does not exist_

```mdtest-spq
# spq
values typename("port")
# input
80
# expected output
error("missing")
```
