## Expressions

Expressions are the means the carry out calculations and utilize familiar
query-language elements like literal values, function calls, subqueries,
and so forth.

Within [pipe operators](../operators/intro.md),
expressions may reference input values either via the special value
`this` or implied field references to `this`, while
within [SQL clauses](../sql/intro.md), input is referenced with table and
column references.

For example, [`values`](../operators/values.md), [`where`](../operators/where.md),
[`cut`](../operators/cut.md), [`put`](../operators/put.md),
[`sort`](../operators/sort.md) and so forth all utilize various expressions
as part of their semantics.

Likewise, the projected columns of a
[`SELECT`](../../sql/select.md) from the very same expression syntax
used by pipe operators.

While SQL expressions and pipe expressions share an identical syntax,
their semantics diverges in some key ways:
* SQL expressions cannot access the special value `this` and pipe expressions have
  no way of referencing tables or column as dataflow scoping and relational scoping
  are mutually exclusive;
* array indexing is 0-based in pipe expressions and [1-based](index.md#sql-semantics)
  in SQL expressions; and
* double-quoted string literals may be used in pipe expressions but are intepreted 
  as identifiers in SQL expression.

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

For identifiers that appear in the context of call syntax, i.e., having the form
```
<id> ( <args> )
```
then `<id>` is either a [built-in function](../functions/intro.md) name,
a reference to a [declared function](../declarations/functions.md) or
a reference to a [declared operator](../declarations/operators.md).
Otherwise, a compile-time error results.

For identifiers that appear in the context of a function reference, i.e., having the form
```
&<id>
```
then <id> is

For other instances of identifiers,
if the identifier does not correspond to a declaration, then it is presumed 
to be an [input reference](inputs.md) and is resolved as such.


Otherwise, the 

Otherwise, if the declared identifier aappears in call syntax, it must be
3a [function](../functions/intro.md) or an
[operator](../declarations/operators.md)
* if it appears using call syntax, e.g., `<id> ( <args> )` then it is resolved 


* otherwise, 
* if the identifier exists in the scope as an [operator](../declarations/operators.md). declaration,
  the identifier is substituted with a [subqyery](subquery.md) invoking
  tha operator,
*   

  XXX explain automatic promotion of operator references to subqueries

### Precedence

XXX TODO

### Coercion

XXX TODO

### Examples

XXX TODO precedence, coercion, and id resolution
