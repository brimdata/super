### Operator

&emsp; **top** &mdash; get top N sorted values of input sequence

### Synopsis

```
top <const-expr> <expr> [, <expr> ...]
```
### Description

The `top` operator returns the top N values from a sequence sorted in descending
order by one or more expressions. N is given by `<const-expr>`, a compile-time
constant expression that evaluates to a positive integer.

`top` is functionally similar to [`sort`](sort.md) but is less resource
intensive because only the top N values are stored in memory (i.e., values
less than the minimum are discarded).

### Examples

_Grab the top two values from a sequence of integers_
```mdtest-spq
# spq
top 2 this
# input
1
5
3
9
23
7
# expected output
23
9
```

_Find the two names most frequently referenced in a sequence of records_
```mdtest-spq
# spq
count() by name | top 2 count
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
{name:"liz",count:3(uint64)}
{name:"bob",count:2(uint64)}
```
