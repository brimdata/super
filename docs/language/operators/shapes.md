### Operator

&emsp; **shapes** &mdash; select one value of each shape

### Synopsis
```
shapes [<expr>]
```
### Description

The `shapes` operator is a syntactic shortcut for
```
val:=any(<expr>) by typeof(<expr>) | values val
```
If `<expr>` is not provided, `this` is used.

In other words, `shapes` produces one value of each type in the input.
This is useful for data exploration when you want to see the shapes
of data and some sample data in a data set without having to sift
through it all to slice and dice it.

### Examples

_A simple sample_
```mdtest-spq
# spq
shapes | sort this
# input
1
2
3
"foo"
"bar"
10.0.0.1
10.0.0.2
# expected output
1
"foo"
10.0.0.1
```

_Sampling record shapes_
```mdtest-spq
# spq
shapes | sort a
# input
{a:1}
{a:2}
{s:"foo"}
{s:"bar"}
{a:3,s:"baz"}
# expected output
{a:1}
{a:3,s:"baz"}
{s:"foo"}
```
