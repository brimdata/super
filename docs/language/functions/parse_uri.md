### Function

&emsp; **parse_uri** &mdash; parse a string URI into a structured record

### Synopsis

```
parse_uri(uri: string) -> record
```

### Description

The _parse_uri_ function parses the `uri` argument that must have the form of a
[Universal Resource Identifier](https://en.wikipedia.org/wiki/Uniform_Resource_Identifier)
into a structured URI comprising the parsed components as a Zed record
with the following type signature:
```
{
  scheme: string,
  opaque: string,
  user: string,
  password: string,
  host: string,
  port: uint16,
  path: string,
  query: |{string:[string]}|,
  fragment: string
}
```

### Examples

```mdtest-spq {data-layout="stacked"}
# spq
yield parse_uri(this)
# input
"scheme://user:password@host:12345/path?a=1&a=2&b=3&c=#fragment"
# expected output
{scheme:"scheme",opaque:null(string),user:"user",password:"password",host:"host",port:12345(uint16),path:"/path",query:|{"a":["1","2"],"b":["3"],"c":[""]}|,fragment:"fragment"}
```
