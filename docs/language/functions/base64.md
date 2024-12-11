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
```mdtest-command
echo '0x010203' | super -z -c 'yield base64(this)' -
```

```mdtest-output
"AQID"
```
Decode "AQID" into byte sequence `0x010203`:
```mdtest-command
echo '"AQID"' | super -z -c 'yield base64(this)' -
```

```mdtest-output
0x010203
```
Encode ASCII string into Base64-encoded string:
```mdtest-command
echo '"hello, world"' | super -z -c 'yield base64(bytes(this))' -
```

```mdtest-output
"aGVsbG8sIHdvcmxk"
```
Decode a Base64 string and cast the decoded bytes to a string:
```mdtest-command
echo '"aGVsbG8gd29ybGQ="' | super -z -c 'yield string(base64(this))' -
```

```mdtest-output
"hello world"
```
