## Indexing

The index operation is denoted with square brackets and can be applied to
various data types having the form:
```
<value> [ <index> ]
```
If the `<value>` expression is a record, then the `<index>` operand
must be coercible to a string and the result is the record's field
of that name.

If the `<value>` expression is an array, then the `<index>` operand
must be coercible to an integer and the result is the
value in the array of that index.

If the `<value>` expression is a set, then the `<index>` operand
must be coercible to an integer and the result is the
value in the set of that index ordered by total order of values.

If the `<value>` expression is a map, then the `<index>` operand
is presumed to be a key and the corresponding value for that key is
the result of the operation.  If no such key exists in the map, then
the result is `error("missing")`.

If the `<value>` expression is a string, then the `<index>` operand
must be coercible to an integer and the result is an integer representing
the unicode code point at that offset in the string.

If the `<value>` expression is type `bytes`, then the `<index>` operand
must be coercible to an integer and the result is an unsigned 8-bit integer
representing the byte value at that offset in the bytes sequence.

### Slices

The slice operation is a variation of indexing that returns a range of
svalues and can be applied to various data types.  A slice has the form:
```
<value> [ <from> : <to> ]
```
The `<from>` and `<to>` terms must be expressions that are coercible
to integers and represent a range of index values to form a subset of elements
from the `<value>` term provided.  The range begins at the `<from>` position
and ends one before the `<to>` position.  A negative
value of `<from>` or `<to>` represents a position relative to the
end of the value being sliced.

If the `<value>` expression is an array, then the result is an array of
elements comprising the indicated range.

If the `<value>` expression is a set, then the result is a set of
elements comprising the indicated range ordered by total order of values.

If the `<value>` expression is a string, then the result is a substring
consisting of unicode code points comprising the given range.

If the `<value>` expression is type `bytes`, then the result is a bytes sequence
consisting of bytes comprising the given range.

Note that if the expression has side effects,
as with [aggregate function calls](expressions.md#aggregate-function-calls), only the selected expression
will be evaluated.

### SQL Semantics

In SQL expressions, array indexing and slicing is 1-based,
meaning the first element of the array is at index `1`
and the last element of a N-element array is at index `N`.

Everywhere else, array indexing and slicing is 0-based,
meaning the first element of the array is at index `0`
and the last element of a N-element array is at index `N-1`.

### Examples

```mdtest-spq
# spq
values this=="foo" ? {foocount:count()} : {barcount:count()}
# input
"foo"
"bar"
"foo"
# expected output
{foocount:1::uint64}
{barcount:1::uint64}
{foocount:2::uint64}
```
