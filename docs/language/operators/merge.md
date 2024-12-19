### Operator

&emsp; **merge** &mdash; combine parallel pipeline branches into a single, ordered output

### Synopsis

```
( => ... => ...) |> merge <expr> [, <expr>, ...]
```
### Description

The `merge` operator merges inputs from multiple upstream branches of
the pipeline into a single output.  The order of values in the combined
output is determined by the `<expr>` arguments, which act as sort expressions
where the values from the upstream pipeline branches are forwarded based on these expressions.

### Examples

_Copy input to two pipeline branches and merge_
```mdtest-command
echo '1 2' | super -z -c 'fork (=>pass =>pass) |> merge this' -
```

```mdtest-output
1
1
2
2
```
