### Aggregate Function

&emsp; **or** &mdash; logical OR of input values

### Synopsis
```
or(bool) -> bool
```

### Description

The _or_ aggregate function computes the logical OR over all of its input.

### Examples

Ored value of simple sequence:
```mdtest-spq
# spq
or(this)
# input
false
true
false
# expected output
true
```

Continuous OR of simple sequence:
```mdtest-spq
# spq
values or(this)
# input
false
true
false
# expected output
false
true
true
```

Unrecognized types are ignored and not coerced for truthiness:
```mdtest-spq
# spq
values or(this)
# input
false
"foo"
1
true
false
# expected output
false
false
false
true
true
```

OR of values grouped by key:
```mdtest-spq
# spq
or(a) by k | sort
# input
{a:true,k:1}
{a:false,k:1}
{a:false,k:2}
{a:false,k:2}
# expected output
{k:1,or:true}
{k:2,or:false}
```
