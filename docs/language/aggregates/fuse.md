### Aggregate Function

&emsp; **fuse** &mdash; compute a fused type of input values

### Synopsis
```
fuse(any) -> type
```

### Description

The _fuse_ aggregate function applies [type fusion](../shaping.md#type-fusion)
to its input and returns the fused type.

It is useful with grouped aggregation for data exploration and discovery
when searching for shaping rules to cluster a large number of varied input
types to a smaller number of fused types each from a set of interrelated types.

### Examples

Fuse two records:
```mdtest-spq
# spq
fuse(this)
# input
{a:1,b:2}
{a:2,b:"foo"}
# expected output
<{a:int64,b:int64|string}>
```

Fuse records with a grouping key:
```mdtest-spq {data-layout="stacked"}
# spq
fuse(this) by b | sort
# input
{a:1,b:"bar"}
{a:2.1,b:"foo"}
{a:3,b:"bar"}
# expected output
{b:"bar",fuse:<{a:int64,b:string}>}
{b:"foo",fuse:<{a:float64,b:string}>}
```
