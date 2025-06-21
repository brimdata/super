### Operator

&emsp; **combine** &mdash; combine parallel pipeline branches into a single output

### Synopsis

```
( ... ) ( ... ) | ...
```
### Description

The implied `combine` operator merges inputs from multiple upstream branches of
the pipeline into a single output.  The order of values in the combined
output is undefined.

You need not explicit reference the operator with any text.  Instead, the
mere existence of a merge point in the flow graph implies its existence
and its semantics of undefined merge order.

### Examples

_Copy input to two pipeline branches and combine with the implied operator_
```mdtest-spq
# spq
fork
  ( pass )
  ( pass )
| sort this
# input
1 2
# expected output
1
1
2
2
```
