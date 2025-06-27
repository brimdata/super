### Function

&emsp; **hex** &mdash; encode/decode hexadecimal strings

### Synopsis

```
hex(b: bytes) -> string
hex(s: string) -> bytes
```

### Description

The _hex_ function encodes a bytes value  `b` as
a hexadecimal string or decodes a hexadecimal string `s` into a bytes value.

### Examples

Encode a simple bytes sequence as a hexadecimal string:
```mdtest-spq
# spq
values hex(this)
# input
0x0102ff
# expected output
"0102ff"
```
Decode a simple hex string:
```mdtest-spq
# spq
values hex(this)
# input
"0102ff"
# expected output
0x0102ff
```
Encode the bytes of an ASCII string as a hexadecimal string:
```mdtest-spq
# spq
values hex(bytes(this))
# input
"hello, world"
# expected output
"68656c6c6f2c20776f726c64"
```

Decode hex string representing ASCII into its string form:
```mdtest-spq
# spq
values string(hex(this))
# input
"68656c6c6f20776f726c64"
# expected output
"hello world"
```
