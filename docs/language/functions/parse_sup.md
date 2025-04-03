### Function

&emsp; **parse_sup** &mdash; parse SUP (formerly known as ZSON) or JSON text into a value

### Synopsis

```
parse_sup(s: string) -> any
```

### Description

The _parse_sup_ function parses the `s` argument that must be in the form
of SUP or JSON into a value of any type.  This is analogous to JavaScript's
`JSON.parse()` function.

### Examples

_Parse SUP text_
```mdtest-spq
# spq
foo := parse_sup(foo)
# input
{foo:"{a:\"1\",b:2}"}
# expected output
{foo:{a:"1",b:2}}
```

_Parse JSON text_
```mdtest-spq
# spq
foo := parse_sup(foo)
# input
{"foo": "{\"a\": \"1\", \"b\": 2}"}
# expected output
{foo:{a:"1",b:2}}
```
