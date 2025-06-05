### Operator

&emsp; **fork** &mdash; copy values to parallel pipeline branches

### Synopsis

```
fork (
  => <branch>
  => <branch>
  ...
)
```
### Description

The `fork` operator copies each input value to multiple, parallel branches of
the pipeline.

The output of a fork consists of multiple branches that must be merged.
If the downstream operator expects a single input, then the output branches are
merged with an automatically inserted [combine operator](combine.md).

### Examples

_Copy input to two pipeline branches and merge_
```mdtest-spq
# spq
fork (
  =>pass
  =>pass
)
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
