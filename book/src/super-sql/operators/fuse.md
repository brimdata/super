### Operator

&emsp; **fuse** &mdash; coerce all input values into a fused type

### Synopsis

```
fuse
```
### Description

The `fuse` operator computes a [fused type](../type-fusion.md)
over all of its input then casts all values in the input to the fused type.

This is logically equivalent to:
```
from input | values cast(this, (from input | aggregate fuse(this)))
```

Because all values of the input must be read to compute the fused type,
`fuse` may spill its input to disk when memory limits are exceeded.

>[!NOTE]
> Spilling is not yet implemented for the vectorized runtime.

### Examples

---

_Fuse two records_
```mdtest-spq
# spq
fuse
# input
{a:1}
{b:2}
# expected output
{a:1,b:null::int64}
{a:null::int64,b:2}
```

---

_Fuse records with type variation_
```mdtest-spq
# spq
fuse
# input
{a:1}
{a:"foo"}
# expected output
{a:1::(int64|string)}
{a:"foo"::(int64|string)}
```

---

_Fuse records with complex type variation_
```mdtest-spq {data-layout="stacked"}
# spq
fuse
# input
{a:[1,2]}
{a:["foo","bar"],b:10.0.0.1}
# expected output
{a:[1,2]::[int64|string],b:null::ip}
{a:["foo","bar"]::[int64|string],b:10.0.0.1}
```
