### Aggregate Function

&emsp; **union** &mdash; set union of input values

### Synopsis
```
union(any) -> |[any]|
```

### Description

The _union_ aggregate function computes a set union of its input values.
If the values are of uniform type, then the output is a set of that type.
If the values are of mixed type, the the output is a set of union of the
types encountered.

### Examples

Create a set of values from a simple sequence:
```mdtest-spq
# spq
union(this)
# input
1
2
3
3
# expected output
|[1,2,3]|
```

Create sets continuously from values in a simple sequence:
```mdtest-spq
# spq
values union(this)
# input
1
2
3
3
# expected output
|[1]|
|[1,2]|
|[1,2,3]|
|[1,2,3]|
```

Mixed types create a union type for the set elements:
```mdtest-spq
# spq
set:=union(this) | values this,typeof(set)
# input
1
2
3
"foo"
# expected output
{set:|[1,2,3,"foo"]|}
<|[int64|string]|>
```

Create sets of values bucketed by key:
```mdtest-spq
# spq
union(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
{a:4,k:2}
# expected output
{k:1,union:|[1,2]|}
{k:2,union:|[3,4]|}
```
