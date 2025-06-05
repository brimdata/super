### Operator

&emsp; **merge** &mdash; combine parallel pipeline branches into a single, ordered output

### Synopsis

```
( => ... => ...) | merge <expr> [asc|desc] [nulls {first|last}] [, <expr> [asc|desc] [nulls {first|last}] ...]]
```
### Description

The `merge` operator merges inputs from multiple upstream branches of
the pipeline into a single output.  The order of values in the combined
output is determined by the `<expr>` arguments, which act as sort expressions
where the values from the upstream pipeline branches are forwarded based on these expressions.

### Examples

_Copy input to two pipeline branches and merge_
```mdtest-spq
# spq
fork (
  =>pass
  =>pass
)
| merge this
# input
1
2
# expected output
1
1
2
2
```
