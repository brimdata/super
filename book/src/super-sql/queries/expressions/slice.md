### Slices

The slice operation can be applied to various data types and has the form:
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

For example,
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
