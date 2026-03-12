# fusecomp

compute a complete fused type of input values

## Synopsis

```
fusecomp(any) -> type
```

## Description

The _fusecomp_ aggregate function applies [type fusion](../type-fusion.md)
to its input and returns the complete fused type.  A complete fused type differs
from the regular fused type as it includes fusion types in the nested type hierarchy
whereever type mixtures were utilized to blend types in the type fusion process.

## Examples

Fuse two records:
```mdtest-spq
# spq
fusecomp(this)
# input
{a:1,b:2}
{a:2,b:"foo"}
# expected output
<fusion({a:int64,b:fusion(int64|string)})>
```
