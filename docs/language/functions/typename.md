### Function

&emsp; **typename** &mdash; look up and return a named type

### Synopsis

```
typename(name: string) -> type
```

### Description

The _typename_ function returns the [type](../../formats/jsup.md#25-types) of the
[named type](../../formats/jsup.md#258-named-type) given by `name` if it exists.  Otherwise, `error("missing")` is returned.

### Examples

Return a simple named type with a string constant argument:
```mdtest-spq
# spq
yield typename("port")
# input
80(port=int16)
# expected output
<port=int16>
```

Return a named type using an expression:
```mdtest-spq
# spq
yield typename(name)
# input
{name:"port",p:80(port=int16)}
# expected output
<port=int16>
```

The result is `error("missing")` if the type name does not exist:
```mdtest-spq
# spq
yield typename("port")
# input
80
# expected output
error("missing")
```
