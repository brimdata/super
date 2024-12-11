### Function

&emsp; **parse_zson** &mdash; parse ZSON or JSON text into a Zed value

### Synopsis

```
parse_zson(s: string) -> any
```

### Description

XXX change this to parse_sup()

The _parse_zson_ function parses the `s` argument that must be in the form
of Super JSON or JSON into a value of any type.  This is analogous to JavaScript's
`JSON.parse()` function.

### Examples

_Parse Super JSON text_

```mdtest-command
echo '{foo:"{a:\"1\",b:2}"}' | super -z -c 'foo := parse_zson(foo)' -
```

```mdtest-output
{foo:{a:"1",b:2}}
```

_Parse JSON text_
```mdtest-command
echo '{"foo": "{\"a\": \"1\", \"b\": 2}"}' |
  super -z -c 'foo := parse_zson(foo)' -
```

```mdtest-output
{foo:{a:"1",b:2}}
```
