### Function

&emsp; **network_of** &mdash; the network of an IP

### Synopsis

```
network_of(val: ip [, mask: ip|int|uint]) -> net
```

### Description

The _network_of_ function returns the network of the IP address given
by `val` as determined by the optional `mask`.  If `mask` is an integer rather
than an IP address, it is presumed to be a network prefix of the indicated length.
If `mask` is omitted, then a class A (8 bit), B (16 bit), or C (24 bit)
network is inferred from `val`, which in this case, must be an IPv4 address.

### Examples

Compute the network address of an IP using an `ip` mask argument:
```mdtest-spq
# spq
yield network_of(this, 255.255.255.128)
# input
10.1.2.129
# expected output
10.1.2.128/25
```

Compute the network address of an IP given an integer prefix argument:
```mdtest-spq
# spq
yield network_of(this, 25)
# input
10.1.2.129
# expected output
10.1.2.128/25
```

Compute the network address implied by IP classful addressing:
```mdtest-spq
# spq
yield network_of(this)
# input
10.1.2.129
# expected output
10.0.0.0/8
```

The network of a value that is not an IP is an error:
```mdtest-spq {data-layout="stacked"}
# spq
yield network_of(this)
# input
1
# expected output
error({message:"network_of: not an IP",on:1})
```

Network masks must be contiguous:
```mdtest-spq {data-layout="stacked"}
# spq
yield network_of(this, 255.255.128.255)
# input
10.1.2.129
# expected output
error({message:"network_of: mask is non-contiguous",on:255.255.128.255})
```
