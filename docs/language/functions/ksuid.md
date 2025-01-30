### Function

&emsp; **ksuid** &mdash; encode/decode KSUID-style unique identifiers

### Synopsis

```
ksuid() -> bytes
ksuid(b: bytes) -> string
ksuid(s: string) -> bytes
```

### Description

The _ksuid_ function either encodes a [KSUID](https://github.com/segmentio/ksuid)
(a byte sequence of length 20) `b` into a Base62 string or decodes
a KSUID Base62 string into a 20-byte Zed bytes value.

If _ksuid_ is called with no arguments, a new KSUID is generated and
returned as a bytes value.

#### Example:

```mdtest-spq {data-layout="stacked"}
# spq
id := ksuid(id)
# input
{id:0x0dfc90519b60f362e84a3fdddd9b9e63e1fb90d1}
# expected output
{id:"1zjJzTWWCJNVrGwqB8kZwhTM2fR"}
```
