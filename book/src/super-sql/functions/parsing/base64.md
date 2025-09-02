### Function

&emsp; **base64** &mdash; encode/decode Base64 strings

### Synopsis

```
base64(b: bytes) -> string
base64(s: string) -> bytes
```

### Description

The `base64` function encodes a bytes value `b` as
a [Base64](https://en.wikipedia.org/wiki/Base64) string,
or decodes a Base64 string `s` into a bytes value.

### Examples

---

_Encode byte sequence `0x010203` into its Base64 string_

```mdtest-spq
# spq
values base64(this)
# input
0x010203
# expected output
"AQID"
```

---

_Decode "AQID" into byte sequence `0x010203`_

```mdtest-spq
# spq
values base64(this)
# input
"AQID"
# expected output
0x010203
```

---

_Encode ASCII string into Base64-encoded string_

```mdtest-spq
# spq
values base64(this::bytes)
# input
"hello, world"
# expected output
"aGVsbG8sIHdvcmxk"
```

---

_Decode a Base64 string and cast the decoded bytes to a string_

```mdtest-spq
# spq
values base64(this)::string
# input
"aGVsbG8gd29ybGQ="
# expected output
"hello world"
```
