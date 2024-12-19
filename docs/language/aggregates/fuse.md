### Aggregate Function

&emsp; **fuse** &mdash; compute a fused type of input values

### Synopsis
```
fuse(any) -> type
```

### Description

The _fuse_ aggregate function applies [type fusion](../shaping.md#type-fusion)
to its input and returns the fused type.

This aggregation is useful with group-by for data exploration and discovery
when searching for shaping rules to cluster a large number of varied input
types to a smaller number of fused types each from a set of interrelated types.

### Examples

Fuse two records:
```mdtest-command
echo '{a:1,b:2}{a:2,b:"foo"}' | super -z -c 'fuse(this)' -
```

```mdtest-output
<{a:int64,b:(int64,string)}>
```
Fuse records with a group-by key:
```mdtest-command
echo '{a:1,b:"bar"}{a:2.1,b:"foo"}{a:3,b:"bar"}' |
  super -z -c 'fuse(this) by b |> sort' -
```

```mdtest-output
{b:"bar",fuse:<{a:int64,b:string}>}
{b:"foo",fuse:<{a:float64,b:string}>}
```
