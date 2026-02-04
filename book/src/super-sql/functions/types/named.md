# named

wrap a value with a named type

## Synopsis

```
named(val: any, name: string) -> any
```

## Description

The `named` function wraps `val` with a [named type](../../types/named.md)
whose name is the `name` parameter and whose underlying type is the type of `val`.
If `val` is already a named type, the existing name is replaced with `name`.

## Examples

---

_Create a named type_

```mdtest-spq
# spq
named(this, "foo")
# input
{a:1,b:2}
{a:3,b:4}
# expected output
{a:1,b:2}::=foo
{a:3,b:4}::=foo
```

---

_Derive type names from the properties of data_

```mdtest-spq
# spq
values named(this, has(x) ? "point" : "radius")
# input
{x:1,y:2}
{r:3}
{x:4,y:5}
# expected output
{x:1,y:2}::=point
{r:3}::=radius
{x:4,y:5}::=point
```
