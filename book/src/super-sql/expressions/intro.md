## Expressions

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
Such variations include:

XXX SQL vs pipe
* access to `this`
* array indexing (0-based vs 1-based)
* identifier references (double quote vs backtick quote)

### Expression Syntax

Expressions are composed from operands and operators over operands.

Operands include
  * [inputs](inputs.md),
  * [literals](literals.md),
  * [formatted strings](f-strings.md)
  * [function calls](functions.md),
  * [aggregate function calls](aggregates.md),
  * [subqueries](subqueries.md), or
  * other expressions.

Operators include
  * [arithmetic](./arithmetic.md) to add, subtract, multiply, divide, etc,
  * [cast](cast.md) to convert values from one type to another,
  * [comparisons](comparisons.md) to compare two values resulting in a Boolean,
  * [conditionals](conditional) including C-style `?-:` operator and SQL `CASE` expressions,
  * [containment](containment.md) to test for the existing value inside an array or set,
  * [dot](dot.md) to access a field of a record (or a SQL column of a table),
  * [indexing](index.md) to select and slice elements from
      an array, record, map, string, or bytes, and
  * [logic](logic.md) to combine predicates using Boolean logic.

### Identifier Resolution

An identifier that appears as an operand in an expression is resolved to
the entity that it represents using lexical scoping.

  XXX explain automatic promotion of operator references to subqueries

### Precedence

XXX TODO

### Coercion

XXX TODO

### Examples

XXX TODO precedence, coercion, and id resolution
