# avg

average (arithmetic mean) value

## Synopsis

```
avg(number) -> number
```

## Description

The _avg_ aggregate function computes the average (arithmetic mean)
value of its input.

## Examples

Average value of simple sequence:
```mdtest-spq
# spq
avg(this)
# input
1
2
3
4
# expected output
2.5
```

Unrecognized types are ignored:
```mdtest-spq
# spq
avg(this)
# input
1
2
3
4
"foo"
# expected output
2.5
```

Average of values bucketed by key:
```mdtest-spq
# spq
avg(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
{a:4,k:2}
# expected output
{k:1,avg:1.5}
{k:2,avg:3.5}
```
