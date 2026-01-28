# parse_sup

parse SUP or JSON text into a value

## Synopsis

```
parse_sup(s: string) -> any
```

## Description

The `parse_sup` function parses the `s` argument that must be in the form
of [SUP](../../../formats/sup.md) or JSON into a value of any type.
This is analogous to JavaScript's `JSON.parse()` function.

## Examples

---

_Parse SUP text_

```mdtest-spq
# spq
foo := parse_sup(foo)
# input
{foo:"{a:\"1\",b:2}"}
# expected output
{foo:{a:"1",b:2}}
```

---

_Parse JSON text_

```mdtest-spq
# spq
foo := parse_sup(foo)
# input
{"foo": "{\"a\": \"1\", \"b\": 2}"}
# expected output
{foo:{a:"1",b:2}}
```
