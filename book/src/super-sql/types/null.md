### Null Type

The `null` type represents a type that has just one value:
the special value null.

A value of type null is formed simply from the keyword `null` 
representing the null value, which by default, is type null.

While all types include a null value, e.g., `null::int64` is the 
null value whose type is `int64`, the null type has no other values
besides the null value.

In relational SQL, a null indicates the oxymoronish concept of the presence
of an absent value.  Nulls arise because relational columns are fixed in
structure and real-world data can be eclectic and not always fit
into predetermined structure.  Null values also arise when dynamic errors
are generated and there is no way to represent a rich error type in the fixed-type
column.

Because SuperSQL has [first-class errors](errors.md) (obviating the need to
serialize error conditions as fixed-type nulls)
and [sum-types](union.md) (obviating the need to flatten sum types into columns and
occupy the absent component types with nulls), the use of null values is
discouraged.

That said, SuperSQL supports the null value for backward compatibility with
their pervasive use in SQL, database systems, programming languages, and serialization
formats.

As in SQL, to test if a value is null, it cannot be compared to another null 
value, which by definition, is always false, i.e., two unknown values cannot
be known to be equal.  Instead the `IS NULL` operator or 
[coalesce](../functions/generics/coalesce.md) function should be used.


#### Examples

_The null value_

> TODO: this f-string is currently broken.  FIX.

```mdtest-spq
# spq
values f"{this} is type {typeof(this)}"
# input
null
# expected output
BUG
```

---
_Test for null with IS NULL_

```mdtest-spq
# spq
values
  this IS NULL,
  this == null,
  this == null ? "null == null" : "null != null"
# input
null
# expected output
true
null::bool
"this != null"
```
---
_Missing values are not null values_

```mdtest-spq
# spq
values {out:y}
# input
{x:1}
{x:2,y:3}
null
# expected output
{out:error("missing")}
{out:3}
{out:error("missing")}
```

---
_Use coalesce to easily skip over nulls and missing values_

```mdtest-spq
# spq
const DEFAULT = 100
values coalesce(y,x,DEFAULT)
# input
{x:1}
{x:2,y:3}
{x:4,y:null}
null
# expected output
1
3
4
100
```
