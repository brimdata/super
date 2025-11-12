### Aggregate Function

&emsp; **and** &mdash; logical AND of input values

### Synopsis
```
and(bool) -> bool
```

### Description

The _and_ aggregate function computes the logical AND over all of its input.

### Examples

Anded value of simple sequence:
```mdtest-spq
# spq
and(this)
# input
true
false
true
# expected output
false
```

Unrecognized types are ignored and not coerced for truthiness:
```mdtest-spq
# spq
and(this)
# input
true
"foo"
0
false
true
# expected output
false
```

AND of values grouped by key:
```mdtest-spq
# spq
and(a) by k | sort
# input
{a:true,k:1}
{a:true,k:1}
{a:true,k:2}
{a:false,k:2}
# expected output
{k:1,and:true}
{k:2,and:false}
```
