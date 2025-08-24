### Bytes

The `bytes` type represents an arbitrary sequence of 8-bit bytes.

The character sequence `0x` followed by an even number of hexadecimal 
digits forms a bytes type.

An empty bytes value is simply `0x` followed by no digits.

For backward compatibility with SQL, `BLOB` is a syntactic alias for type `bytes`.

> _The `BLOB` type alias is not yet implemented in SuperSQL._

#### Examples
---
```mdtest-spq
# spq
values
  0x0102beef,
  'hello, world'::bytes,
  len(0x010203),
  0x,
  null::bytes
# input
null
# expected output
0x0102beef
0x68656c6c6f2c20776f726c64
3
0x
null::bytes
```
