## Inputs

Input data is processed by queries through expressions that contain
input-data references.

In [pipe scoping](../intro.md#pipe-scoping), input data
is always referenced as the special value `this`.

In [relational scoping](../intro.md#relational-scoping), input data
is referenced by specifying the columns of one or more tables.
See the [SQL section](../sql/intro.md#input-references) for
details on how columns are bound to identifiers, how table references
are resolved, and how `this` behaves in a SQL expression.

The type of `this` may be any [type](../types/intro.md).
When `this` is a [record](../types/record.md), references
to fields of the record may be referenced by an identifier that names the
field of the implied `this` value, e.g., `x` means `this.x`.

For expressions that appear in a [SQL operator](../sql/intro.md),
input is presumed to be in the form of records and is referenced using
[relational scoping](../intro.md#relational-scoping).
Here, identifiers refer to table aliases and/or column names
and are bound to the available inputs based on SQL semantics.
When the input schema is known, these references are
statically checked and compile-time errors are raised when invalid
tables or columns are referenced.

In a SQL operator, if the input is not a record (i.e., not relational),
then the input data can still be referred to as the value `this` and placed
into an output relation using [SELECT](../sql/select.md).
When referring to non-relational with `*`, there are no input columns and
thus the select value is empty, i.e., the value `{}`.

When non-record data is referenced in a SQL operator and the input
schema is dynamic and unknown, runtime errors like `error("missing")`
will generally arise and be present in the output data.

### Examples

---

_A simple reference to this_

```mdtest-spq
# spq
values this
# input
1
true
"foo"
# expected output
1
true
"foo"
```

---

_Referencing a field of this_

```mdtest-spq
# spq
values this.x
# input
{x:1,y:4}
{x:2,y:5}
{x:3,y:6}
# expected output
1
2
3
```

---

_Referencing an implied field of this_

```mdtest-spq
# spq
values x
# input
{x:1,y:4}
{x:2,y:5}
{x:3,y:6}
# expected output
1
2
3
```

---

_Selecting a column of an input table in a SQL operator_

```mdtest-spq
# spq
SELECT x
# input
{x:1,y:4}
{x:2,y:5}
{x:3,y:6}
# expected output
{x:1}
{x:2}
{x:3}
```

---

_Selecting a column of an named input table_

```mdtest-spq
# spq
let input = (
  values
    {x:1,y:4},
    {x:2,y:5},
    {x:3,y:6}
)
SELECT x FROM input
# input
null
# expected output
{x:1}
{x:2}
{x:3}
```
