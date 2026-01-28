# collect

aggregate values into array

## Synopsis

```
collect(any) -> [any]
```

## Description

The _collect_ aggregate function organizes its input into an array.
If the input values vary in type, the return type will be an array
of union of the types encountered.

## Examples

Simple sequence collected into an array:
```mdtest-spq
# spq
collect(this)
# input
1
2
3
4
# expected output
[1,2,3,4]
```

Mixed types create a union type for the array elements:
```mdtest-spq
# spq
collect(this) | values this,typeof(this)
# input
1
2
3
4
"foo"
# expected output
[1,2,3,4,"foo"]
<[int64|string]>
```

Create arrays of values bucketed by key:
```mdtest-spq
# spq
collect(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
{a:4,k:2}
# expected output
{k:1,collect:[1,2]}
{k:2,collect:[3,4]}
```
