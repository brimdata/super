# fuse

compute a complete fused type of input values

## Synopsis

```
fuse(any) -> type
```

## Description

The _fuse_ aggregate function applies [type fusion](../type-fusion.md)
to its input and returns the fused type.  A fused type differs
from a [blended type](blend.md) as it includes fusion types in the nested type hierarchy
whereever type changes were made to combine types in the type fusion process.

## Examples

Fuse two records:
```mdtest-spq
# spq
fuse(this)
# input
{a:1,b:2}
{a:2,b:"foo"}
# expected output
<{a:int64,b:fusion(int64|string)}>
```
