# Slices

A slice expression is a variation of an [index](index.md) expression
that returns a range of values instead of a single value and can be applied
to sliceable data types.  A slice has the form
```
<entity> [ <from> : <to> ]
```
where `<entity>` is an [expression](intro.md) that returns an sliceable value
and `<from>` and `<to>` are expressions that are coercible to integers.

Sliceable entities include
[arrays](../types/array.md),
[sets](../types/set.md),
[strings](../types/string.md), and
[bytes](../types/bytes.md).

The value `<from>` and `<to>` represent a range of index values
to form a subset of elements from the `<entity>` term provided.
The range begins at the `<from>` position and ends one element before
the `<to>` position.  A negative value of `<from>` or `<to>` represents
a position relative to the end of the value being sliced.

If the `<entity>` expression is an array, then the result is an array of
elements comprising the indicated range.

If the `<entity>` expression is a set, then the result is a set of
elements comprising the indicated range ordered by total order of values.

If the `<entity>` expression is a string, then the result is a substring
consisting of unicode code points comprising the given range.

If the `<entity>` expression is type `bytes`, then the result is a bytes sequence
consisting of bytes comprising the given range.

## Index Base

The index base for slice expressions is determined identically to
the [index base for indexing](index.md#index-base).
By default, slice indexes are zero based.

## Examples

---

_Simple slices_

```mdtest-spq
# spq
values a[1:3]
# input
{a:[1,2,3,4]}
{a:|[1,2,3,4]|}
{a:"1234"}
{a:0x01020304}
# expected output
[2,3]
|[2,3]|
"23"
0x0203
```

---

_1-based slices_

```mdtest-spq
# spq
pragma index_base = 1
values a[1:3]
# input
{a:[1,2,3,4]}
{a:|[1,2,3,4]|}
{a:"1234"}
{a:0x01020304}
# expected output
[1,2]
|[1,2]|
"12"
0x0102
```

---

_Prefix and suffix slices_

```mdtest-spq
# spq
values {prefix:a[:2],suffix:a[-2:-1]}
# input
{a:[1,2,3,4]}
{a:|[1,2,3,4]|}
{a:"1234"}
{a:0x01020304}
# expected output
{prefix:[1,2],suffix:[3]}
{prefix:|[1,2]|,suffix:|[3]|}
{prefix:"12",suffix:"3"}
{prefix:0x0102,suffix:0x03}
```
