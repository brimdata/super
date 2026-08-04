# array_agg

aggregate values into array

## Synopsis

```
array_agg(any) -> [any]
```

## Description

The _array_agg_ aggregate function organizes its input into an array.
If the input values vary in type, the return type will be an array
of union of the types encountered.

>[!NOTE]
>_array_agg_ is like [collect](collect.md) but matches PostgreSQL in returning
>`null`, rather than an empty array, when no values are aggregated.

## Examples

Simple sequence aggregated into an array (see [collect](collect.md) docs for more examples):
```mdtest-spq
# spq
array_agg(this)
# input
1
2
3
4
# expected output
[1,2,3,4]
```

Contrast `array_agg` with `collect` when no values have been aggregated:
```mdtest-spq
# spq
aggregate
  array_agg(a) filter (a > 1),
  collect(a) filter (a > 1)
  by k
| sort
# input
{a:1,k:1}
{a:2,k:2}
{a:3,k:2}
# expected output
{k:1,array_agg:null,collect:[]}
{k:2,array_agg:[2,3],collect:[2,3]}
```
