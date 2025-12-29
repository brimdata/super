### Operator

[🔀](../intro.md#data-order)&emsp; **top** &mdash; output the first N sorted values of input sequence

### Synopsis

```
top [-r] [<const-expr> [<expr> [asc|desc] [nulls {first|last}] [, <expr> [asc|desc] [nulls {first|last}] ...]]]
```
### Description

The `top` operator returns the first N values from a sequence sorted according
to the provided sort expressions. N is given by `<const-expr>`, a compile-time
constant expression that evaluates to a positive integer. If `<const-expr>` is
not provided, N defaults to `1`.

The sort expressions `<expr>` and their parameters behave as they
do for [`sort`](sort.md). If no sort expression is provided, a sort key is
selected using the same heuristic as [`sort`](sort.md).

`top` is functionally similar to [`sort`](sort.md) but is less resource
intensive because only the first N values are stored in memory (i.e., subsequent
values are discarded).

### Examples

---

_Grab the smallest two values from a sequence of integers_
```mdtest-spq
# spq
top 2
# input
1
5
3
9
23
7
# expected output
1
3
```

---

_Find the two names most frequently referenced in a sequence of records_
```mdtest-spq
# spq
count() by name | top -r 2 count
# input
{name:"joe", age:22}
{name:"bob", age:37}
{name:"liz", age:25}
{name:"bob", age:18}
{name:"liz", age:34}
{name:"zoe", age:55}
{name:"ray", age:44}
{name:"sue", age:41}
{name:"liz", age:60}
# expected output
{name:"liz",count:3}
{name:"bob",count:2}
```
