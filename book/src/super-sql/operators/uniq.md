# uniq

[✅](../intro.md#data-order)&ensp; deduplicate adjacent values

## Synopsis

```
uniq [-c]
```

## Description

Inspired by the traditional Unix shell command of the same name,
the `uniq` operator copies its input to its output but removes duplicate values
that are adjacent to one another.

This operator is most often used with [`cut`](cut.md) and [`sort`](sort.md) to find and eliminate
duplicate values.

When run with the `-c` option, each value is output as a record with the
type signature `{value:any,count:int64}`, where the `value` field contains the
unique value and the `count` field indicates the number of consecutive duplicates
that occurred in the input for that output value.

## Examples

---

_Simple deduplication_
```mdtest-spq
# spq
uniq
# input
1
2
2
3
# expected output
1
2
3
```

---

_Simple deduplication with count of duplicate values_
```mdtest-spq
# spq
uniq -c
# input
1
2
2
3
# expected output
{value:1,count:1}
{value:2,count:2}
{value:3,count:1}
```

---

_Use sort to deduplicate non-adjacent values_
```mdtest-spq
# spq
sort | uniq
# input
"hello"
"world"
"goodbye"
"world"
"hello"
"again"
# expected output
"again"
"goodbye"
"hello"
"world"
```

---

_Complex values must match fully to be considered duplicate (e.g., every field/value pair in adjacent records)_
```mdtest-spq {data-layout="stacked"}
# spq
uniq
# input
{ts:2024-09-10T21:12:33Z, action:"start"}
{ts:2024-09-10T21:12:34Z, action:"running"}
{ts:2024-09-10T21:12:34Z, action:"running"}
{ts:2024-09-10T21:12:35Z, action:"running"}
{ts:2024-09-10T21:12:36Z, action:"stop"}
# expected output
{ts:2024-09-10T21:12:33Z,action:"start"}
{ts:2024-09-10T21:12:34Z,action:"running"}
{ts:2024-09-10T21:12:35Z,action:"running"}
{ts:2024-09-10T21:12:36Z,action:"stop"}
```
