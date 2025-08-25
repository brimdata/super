### Sets

Sets conform to the
[set type](../../formats/model.md#23-set)
in the super-structured data model and follow the
[syntax](../../formats/sup.md#253-set-type)
of sets in the [SUP format](../../formats/sup.md), i.e.,
a set type has the form
```
|[ <type> ]|
```
where `<type>` is any type.

Any SUP text defining a [set value](../../formats/sup.md#243-set-value)
is a valid set literal in the SuperSQL language.

For example, this is a simple set value
```
|[1,2,3]|
```
whose type is
```
|[int64]|
```

An empty set value has the form `|[]|` and
an empty set type defaults to an set of type null, i.e., `|[null]|`,
unless otherwise cast, e.g., `|[]|::[int64]` represents an empty set
of integers.

Sets can be created by reading external data (SUP files,
database data, Parquet values, JSON objects, etc) or by
constructing instances using
[_set expressions_](#set-expressions) or other
SuperSQL functions that produce sets.

#### Set Expressions

Set values are constructed from a _set expression_ that is comprised of
zero or more comma-separated elements contained in pipe brackets:
```
|[ <element>, <element>, ... ]|
```
where an `<element>` has one of two forms:
```
<expr>
```
or
```
...<expr>
```
`<expr>` may be any valid [expression](../expressions.md).

The first form is simply an element in the set, the result of `<expr>`.

The second form is the set spread operator `...`,
which expects an array or set value as
the result of `<expr>` and inserts all of the values from the result.  If a spread
expression results in neither an array nor set, then the value is elided.

When the expressions result in values of non-uniform type, then the types of the
set elements become a sum type of the types present,
tied together with the corresponding [union type](union.md).


#### Examples
---
```mdtest-spq
# spq
values |[3,1,2]|,|["hello","world","hello"]|
# input
null
# expected output
|[1,2,3]|
|["hello","world"]|
```
---

_Arrays and sets can be concatenated using the spread operator_
```mdtest-spq
# spq
values |[...a,...b,4]|
# input
{a:[1,2],b:|[2,3]|}
# expected output
|[1,2,3,4]|
```

---

```mdtest-spq
# spq
values [1,2,3],["hello","world"]
# input
null
# expected output
[1,2,3]
["hello","world"]
```

---

_Sets with mixed types are tied together with a union type_
```mdtest-spq
# spq
values typeof(|[1,"foo"]|)
# input
null
# expected output
<|[int64|string]|>
```

---

_The union aggregate function builds a set
using a sum type for the mixed-type elements_
```mdtest-spq
# spq
union(this) | values this, typeof(this)
# input
1
2
3
1
"hello"
"world"
"hello"
# expected output
|[1,2,3,"hello","world"]|
<|[int64|string]|>
```
