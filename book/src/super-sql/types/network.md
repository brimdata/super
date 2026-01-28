# Networks/IPs

The `ip` type represents an internet address and supports both
IPv4 and IPv6 variations.  The `net` type represents an `ip` value
with a contiguous network mask as indicated by the number of bits
in the mask.

For backward compatibility with SQL, syntactic aliases for signed integers
are defined as follows:
* `CIDR` maps to `net`
* `INET` maps to `ip`

A 32-bit IPv4 address is formed using dotted-decimal notation, e.g.,
a string of base-256 decimal numbers separated by `.` as in
`128.32.130.100` or `10.0.0.1`.

A 128-bit IPv6 is formed from a sequence of eight groups of four
hexadecimal digits separated by colons (`:`).

For IPv6 addresses,
leading zeros in each group can be omitted (e.g., the sequence `2001:0db8`
becomes `2001:db8`) and consecutive groups of zeros can be compressed
using a double colon (`::`) but this can only be done once to avoid ambiguity, e.g.,
```
2001:0db8:0000:0000:0000:0000:0000:0001
```
can be expressed as `2001:db8::1`.

A value of type `net` is formed as an IPv4 or IPv6 address followed by a slash (`/`)
followed by a decimal integer indicating the numbers of bits of contiguous network as
in `128.32.130.100/24` or `fc00::/7`.

Note that unlike other SQL dialects that require IP addresses and networks to be formatted
inside quotation marks, SuperSQL treats these data types as first-class elements that
need not be quoted.

## Examples
---
```mdtest-spq
# spq
values
  128.32.130.100,
  10.0.0.1,
  ::2001:0db8,
  2001:0db8:0000:0000:0000:0000:0000:0001
| values this, typeof(this)
# input

# expected output
128.32.130.100
<ip>
10.0.0.1
<ip>
::2001:db8
<ip>
2001:db8::1
<ip>
```
---

```mdtest-spq
# spq
values 128.32.130.100/24, fc00::/7
| values this, typeof(this)
# input

# expected output
128.32.130.0/24
<net>
fc00::/7
<net>
```
