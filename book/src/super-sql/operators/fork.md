# fork

[🎲](../intro.md#data-order)&ensp; copy values to parallel pipeline branches

## Synopsis

```
fork
  ( <branch> )
  ( <branch> )
  ...
```

## Description

The `fork` operator copies each input value to multiple, parallel branches of
the pipeline.

The output of a fork consists of multiple branches that must be merged.
If the downstream operator expects a single input, then the output branches are
combined without preserving order.  Order may be reestablished by applying a
[`sort`](sort.md) at the merge point.

## Examples

---

_Copy input to two pipeline branches and merge_
```mdtest-spq
# spq
fork
  ( pass )
  ( pass )
| sort this
# input
1
2
# expected output
1
1
2
2
```
