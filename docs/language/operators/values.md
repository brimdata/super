### Operator

&emsp; **values** &mdash; emit values from expressions

### Synopsis

```
[values] <expr> [, <expr>...]
```
### Description

The `values` operator produces output values by evaluating one or more
expressions on each input value and sending each result to the output
in left-to-right order.  Each `<expr>` may be any valid
[expression](../expressions.md).

The `values` keyword is optional since it is an
[implied operator](../pipeline-model.md#implied-operators).

### Examples

_Hello, world_
```mdtest-spq
# spq
values "hello, world"
# input
null
# expected output
"hello, world"
```

_Values evaluates each expression for every input value_
```mdtest-spq
# spq
values 1,2
# input
null
null
null
# expected output
1
2
1
2
1
2
```

_Values typically operates on its input_
```mdtest-spq
# spq
values this*2+1
# input
1
2
3
# expected output
3
5
7
```

_Values is often used to transform records_
```mdtest-spq
# spq
Values [a,b],[b,a] | collect(this)
# input
{a:1,b:2}
{a:3,b:4}
# expected output
[[1,2],[2,1],[3,4],[4,3]]
```
