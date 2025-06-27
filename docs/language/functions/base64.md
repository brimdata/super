### Function

&emsp; **base64** &mdash; encode/decode Base64 strings

### Synopsis

```
base64(b: bytes) -> string
base64(s: string) -> bytes
```

### Description

The _base64_ function encodes a bytes value `b` as a
a [Base64](https://en.wikipedia.org/wiki/Base64) string,
or decodes a Base64 string `s` into a bytes value.

### Examples

Encode byte sequence `0x010203` into its Base64 string:
```mdtest-spq
# spq
values base64(this)
# input
0x010203
# expected output
"AQID"
```

Decode "AQID" into byte sequence `0x010203`:
```mdtest-spq
# spq
values base64(this)
# input
"AQID"
# expected output
0x010203
```

Encode ASCII string into Base64-encoded string:
```mdtest-spq
# spq
values base64(bytes(this))
# input
"hello, world"
# expected output
"aGVsbG8sIHdvcmxk"
```

Decode a Base64 string and cast the decoded bytes to a string:
```mdtest-spq
# spq
values string(base64(this))
# input
"aGVsbG8gd29ybGQ="
# expected output
"hello world"
```
