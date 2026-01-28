# hex

encode/decode hexadecimal strings

## Synopsis

```
hex(b: bytes) -> string
hex(s: string) -> bytes
```

## Description

The `hex` function encodes a bytes value  `b` as
a hexadecimal string or decodes a hexadecimal string `s` into a bytes value.

## Examples

---

_Encode a simple bytes sequence as a hexadecimal string_

```mdtest-spq
# spq
values hex(this)
# input
0x0102ff
# expected output
"0102ff"
```

---

_Decode a simple hex string_

```mdtest-spq
# spq
values hex(this)
# input
"0102ff"
# expected output
0x0102ff
```

---

_Encode the bytes of an ASCII string as a hexadecimal string_

```mdtest-spq
# spq
values hex(this::bytes)
# input
"hello, world"
# expected output
"68656c6c6f2c20776f726c64"
```

---

_Decode hex string representing ASCII into its string form_

```mdtest-spq
# spq
values hex(this)::string
# input
"68656c6c6f20776f726c64"
# expected output
"hello world"
```
