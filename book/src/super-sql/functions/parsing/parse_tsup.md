# parse_tsup

parse TSUP or JSON text into a value

## Synopsis

```
parse_tsup: string) -> any
```

## Description

The `parse_tsup` function parses the `s` argument that must be in the form
of [TSUP](../../../formats/tsup.md) or JSON into a value of any type.
This is analogous to JavaScript's `JSON.parse()` function.

## Examples

---

_Parse TSUP text_

```mdtest-spq
# spq
foo := parse_tsup(foo)
# input
{foo:"{a:\"1\",b:2}"}
# expected output
{foo:{a:"1",b:2}}
```

---

_Parse JSON text_

```mdtest-spq
# spq
foo := parse_tsup(foo)
# input
{"foo": "{\"a\": \"1\", \"b\": 2}"}
# expected output
{foo:{a:"1",b:2}}
```
