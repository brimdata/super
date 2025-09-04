## Expressions

As in SQL and typical programming languages, 
SuperSQL expressions perform calculations, logical comparisons,
data manipulation, complex value creation, and so forth.

Expressions are used within [pipe operators](../operators/index.md)
and [SQL clauses](../sql/index.md) to perform computation
utilizing an operator's input values, literal values, function calls, and
subqueries.

For example, [`values`](../operators/values.md), [`where`](../operators/where.md),
[`cut`](../operators/cut.md), [`put`](../operators/put.md),
[`sort`](../operators/sort.md) and so forth all utilize various expressions
as part of their semantics.

Likewise, the projected columns of a
[`SELECT`](../../sql/select.md) from the very same expression syntax
(though with some variation in semantics) used by pipe operators.

### Syntax

Expressions are built up from basic values and operators over values.

Basic values include 
  * [input values](#input-values),
  * [literals](#literals), or
  * [subquery](../subqueries.md) results.

Operators include
  * [aggregate functions](./aggregates.md) to carry out running aggregations using
      any available [aggregate function](../../aggregates/intro.md),
  * [arithmetic](./arithmetic.md) to add, subtract, multiply, divide, etc,
  * [cast](./cast.md) expressions convert values from one type to another,
  * [comparisons](./comparisons.md) to compare two values resulting in a Boolean,
  * [conditionals](./conditional) including C-style `?-:` operator and SQL `CASE` expressions,
  * [containment](./containment.md) to test for the existing value inside an array or set,
  * [f-strings](./f-strings.md) to easily compute values from expressions embedded inside strings,
  * [functions](./functions.md) to apply [built-in functions](../../functions/intro.md) or
    [user functions](../user-functions.md) to zero or more input arguments
    producing one value as a result,
  * [index](./index.md) operator to select and slice elements from an array, record, or map,
  * [logic](./logic.md) operators to combine predicates using Boolean logic.

### Input Values

For expressions that appear in pipe operators,
input is referenced using [dataflow scoping](../../intro.md#dataflow-scoping),
where all input is referenced as a single value called `this`.

The type of `this` may be any [type](../../types/intro.md).
When `this` is a [record](../../types/record.md), references
to fields of the record may be referenced by an indentifier that names the
field of the implied `this` value, e.g., `x` means `this.x`.

For expressions that appear in a [SQL operator](../../sql/intro.md),
input is presumed to be in the form of records and is referenced using
[relational scoping](../../intro.md#relational-scoping).
Here, identifiers refer to table aliases and/or column names
and are bound to the available inputs based on SQL semantics.
When the input schema is known and is static, these references are
statically checked and compile-time errors are raised when unknown
tables or columns are referenced.

> _In a future version of SuperSQL, static type analysis will also
> apply to input references in pipe operators using super-structured
> type analysis instead of relational schemas._

When non-record data is referenced in a SQL operator and the input
schema is dynamic and unknown, structured errors will generally arise
and be present in the output data.  These errors can be used to debug
the problem as the offending values will be present in the `on` field
of the structured errors.

XXX actually these will usually be error missing...

### Field Dereference

Record fields are dereferenced with the dot operator `.` as is customary
in other languages and have the form
```
<value> . <id>
```
where `<id>` is an identifier representing the field name referenced.
If a field name is not representable as an identifier, then [indexing](#indexing)
may be used with a quoted string to represent any valid field name.
Such field names can be accessed using
[`this`](pipeline-model.md#the-special-value-this) and an array-style reference, e.g.,
`this["field with spaces"]`.

XXX Backtick-escaped identifier

If the dot operator is applied to a value that is not a record
or if the record does not have the given field, then the result is
`error("missing")`.

### Literals

Literal values represent specific instances of a type embedded directly
into an expression like the integer `1`, the record `{x:1.5,y:-4.0}`,
or the mixed-type array `[1,"foo"]`.

Any valid [SUP](../../../formats/sup.md) serialized text is a valid literal in SuperSQL.
In particular, complex-type expressions composed recursively of
other literal values can be used to construct any complex literal value,
e.g.,
* [record expressions](../../types/record.md#record-expressions),
* [array expressions](../../types/array.md#array-expressions),
* [set expressions](../../types/set.md#set-expressions),
* [map expressions](../../types/map.md#map-expressions), and
* [error expressions](../../types/error.md).

Literal values of types
[enum](../../types/enum.md),
[union](../../types/union.md), and
[named](../../types/named.md)
may be created with a [cast](cast.md).

### Semantic Variation

To suppport SQL compatibility while also allowing for modern lanuage semantics

SQL vs pipe
* access to `this`
* array indexing (0-based vs 1-based)
* identifier references (double quote vs backtick quote)

There is no `this` in a select expression, for the curious
```
super -C "select ..."
```
or `super compile`
