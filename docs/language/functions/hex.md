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
```mdtest-command
echo '0x0102ff' | super -z -c 'yield hex(this)' -
```

```mdtest-output
"0102ff"
```
Decode a simple hex string:
```mdtest-command
echo '"0102ff"' | super -z -c 'yield hex(this)' -
```

```mdtest-output
0x0102ff
```
Encode the bytes of an ASCII string as a hexadecimal string:
```mdtest-command
echo '"hello, world"' | super -z -c 'yield hex(bytes(this))' -
```

```mdtest-output
"68656c6c6f2c20776f726c64"
```
Decode hex string representing ASCII into its string form:
```mdtest-command
echo '"68656c6c6f20776f726c64"' | super -z -c 'yield string(hex(this))' -
```

```mdtest-output
"hello world"
```
