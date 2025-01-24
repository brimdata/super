### Function

&emsp; **parse_zson** &mdash; parse Super JSON (formerly known as ZSON) or JSON text into a value

### Synopsis

```
parse_zson(s: string) -> any
```

### Description

XXX change this to `parse_jsup()`

The _parse_zson_ function parses the `s` argument that must be in the form
of Super JSON or JSON into a value of any type.  This is analogous to JavaScript's
`JSON.parse()` function.

### Examples

_Parse Super JSON text_
```mdtest-spq
# spq
foo := parse_zson(foo)
# input
{foo:"{a:\"1\",b:2}"}
# expected output
{foo:{a:"1",b:2}}
```

_Parse JSON text_
```mdtest-spq
# spq
foo := parse_zson(foo)
# input
{"foo": "{\"a\": \"1\", \"b\": 2}"}
# expected output
{foo:{a:"1",b:2}}
```
