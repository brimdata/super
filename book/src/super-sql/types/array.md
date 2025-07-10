### Arrays

The array type follows the definition of the
[array type](../../formats/model.md#21-array) from the 
super-structured data model.

Arrays can be created by reading external data (SUP files, 
database data, Parquet values, JSON objects, etc) or by 
constructing instances using
[array expressions](#array-expressions) or other 
SuperSQL functions that produce arrays.

Any SUP text defining an [array value](../../formats/sup.md#241-array-value)
is a valid array literal in the SuperSQL language.

#### Array Expressions

Array values are constructed from an _array expression_ that is comprised of
zero or more comma-separated elements contained in brackets:
```
[ <element>, <element>, ... ]
```
where an `<element>` has one of two forms:
```
<expr>
```
or
```
...<expr>
```
and where `<expr>` is any valid [expression](../expressions.md).

The first form is simply an element in the array, the result of `<expr>`.

The second form is the array spread operator `...`,
which expects an array or set value as
the result of `<expr>` and inserts all of the values from the result.  If a spread
expression results in neither an array nor set, then the value is elided.

When the expressions result in values of non-uniform type, then the type of the
array elements become a sum type of the types present,
tied together with the corresponding [union type](union.md).

An empty array value has the form `[]`.

#### Array Type

An array type has the syntax defined for the
[array type](../../formats/sup.md#251-record-type)
in the [SUP format](../../formats/sup/md), i.e.,
```
[ <type> ]
```
where `<type>` is any type.

An empty array type defaults to an array of type null, i.e., `[null]`.

#### Examples


```mdtest-spq
# spq
values [1,2,3],["hello","world"]
# input
null
# expected output
[1,2,3]
["hello","world"]
```

Arrays can be concatenated using the spread operator:
```mdtest-spq
# spq
values [...a,...b,5]
# input
{a:[1,2],b:[3,4]}
# expected output
[1,2,3,4,5]
```

Arrays with mixed type are tied together with a union type:
```mdtest-spq
# spq
values typeof([1,"foo"])
# input
null
# expected output
<[int64|string]>
```
