# SQL

SuperSQL is backward compatible with SQL in that any SQL query
is a SuperSQL [pipe operator](../operators/intro.md).
A SQL query used as a pipe operator in SuperSQL is called a _SQL pipe operator_.

A SQL query typically specifies its input using one or more
[FROM](from.md) clauses, but when the FROM clause is omitted,
the query takes its input from the parent operator.
If there is no parent operator and `FROM` is omitted, then the
default input is a single `null` value.

A `FROM` clause may also take input from its parent when using
an [f-string](../expressions/f-strings.md) as its input table.
In this case, the input table is dynamically typed.

## Query Envelope

A SQL pipe operator is a query having the form of a `<query-envelope>` defined as
```
[ <with> ]
<query-body>
[ <order-by> ]
[ <limit> ]
```
where
* `<with>` is an optional [WITH](with.md) clause containing one or more
   comma-separated common table-expressions (CTEs),
* `<query-body>` is a recursively defined query structure as defined below,
* `<order-by>` is an optional list of one or more sort expressions
   in an [ORDER BY](order.md) clause, and
* `<limit>` is a [LIMIT](limit.md) clause constraining the number of rows in the output.

Query envelopes produce relational data in the form of sets of records
and may appear in several contexts including:
* the top-level query,
* as a data-source element of a [FROM](from.md) clause,
* as a data-source element of [JOIN](join.md) clause embedded in a [FROM](from.md) clause,
* as a [subquery](../expressions/subqueries.md) in expressions, and
* as operands in a [set operation](#set-operators).

Note that all of the elements of a `<query-envelope>` are optional except the
`<query-body>`.  Thus, any form of a simple `<query-body>` may appear
anywhere a `<query-envelope>` may appear.

> [!NOTE]
> The `WINDOW` clause not yet available in SuperSQL.

## Query Body

The `<query-body>` has one of the following forms:
* a [SELECT](select.md) clause,
* a [VALUES](values.md) clause,
* a [set operation](#set-operators), or
* a parenthesized query of the form `( <query-envelope> )`.

## Set Operators

Set operators combine two input relations using set union, set intersection,
and set substraction.

A set operation has the form:
```
<query-envelope> UNION [ALL | DISTINCT] <query-envelope>
<query-envelope> INTERSECT [ALL | DISTINCT] <query-envelope>
<query-envelope> EXCLUDE [ALL | DISTINCT] <query-envelope>
```
where `<query-envelope>` is a query as [defined above](#query-envelope).

These binary operators are left associative.  Parenthesized queries
may be used to override the default left-to-right evaluation order.

Currently, only the [UNION](union.md) set operator is supported.

>[!NOTE]
> The `INTERSECT` AND `EXCLUDE` operators are not yet available in SuperSQL.

## Table Structure

While data in SuperSQL need not conform to fixed schemas, the
relational operators of SQL that are implemented within SQL pipe operators
presume input in the form of tables.  Even though input may not
always be tables, the entities processed by and produced by SQL pipe
operators are referred to with table-centric terminology, e.g., tables,
table aliases, columns, column aliases, and so forth.

When SQL pipe operators encounter data that is not in table form,
errors typically arise, e.g., compile-time errors indicating a query
referencing non-existent columns or in the case of dynamic inputs,
runtime errors arise indicating absent columns with `error("missing")`.

SuperSQL tables allow for heterogeneity of column type but presume
all inputs to be records.  when a column reference is valid because
some row has the indicated field, rows that do not have the column
result in `error("missing")`

> [!NOTE]
> When querying highly heterogenous data (e.g., JSON events),
> it is usually desirable to use [pipe operators](../operators/intro.md)
> on arbitrary data instead of SQL queries on tables.

## Table and Column References

Identifiers that appear in SQL expressions are resolved in accordance
with the relational model, typically referring to tables and columns and by name.

Each SQL pipe operator defines one or more relational namespaces
that are independent of other SQL pipe operators and does not span across
pipe operator boundaries.

A [FROM](from.md) clause creates a relational scope defined by a
namespace comprising one or more table names each containing
one or more column names according to the
scoping rules of the `FROM` body.

A particular column is referenced by name using the syntax
```
<column>
```
or
```
<table> . <column>
```
where `<table>` and `<column>` are identifiers.
The first form is called an _unqualified column reference_ while the
second form is called a _qualified column reference_.

Note that the `.` operator here is overloaded as it is used both (1) to indicate
a column inside of a table and (2) to [derefence a record value](../expressions/dot.md),
which means that the syntax
```
<name> . <name>
```
where `<name>` is an identifier, can mean either
```
<column> . <field>
```
when the specified column is a record type or
```
<table> . <column>
```
when the first name resolves to a table.
As described below, when tables and columns have identical names,
the name resolution for columns has precedence over tables.

A table referenced without a column qualifier, as in
```
<table>
```
is simply called a _table reference_.  Table references within expressions
result in values that comprise the entire row of the table as a record.

## Identifier Resolution

Identifiers that correspond to entities other than tables and columns,
e.g., function names are declarations,
have precedence over table and column references and are resolved as follows:

* If the identifier appears in a call expression, then it
  [binds to a function call](../expressions/intro.md#identifier-resolution)
  as in pipe scope.
* If the identifier corresponds to an in-scope
  [constant](../declarations/constants.md),
  [type](../declarations/types.md), or
  [query](../declarations/queries.md) declaration, then it
  [binds to that declaration](../expressions/intro.md#identifier-resolution)
  as in pipe scope.

When an identifier does not resolve as above,
then it is resolved as a table or column reference as follows:

* If the identifier corresponds to a column defined in a table
  of _reachable scope_ (see below), then it binds to the column of that table
  provided that the identifier resolve to no other columns in other reachable tables;
  if multiple tables match, then an error is reported and the query fails to compile.
* Otherwise, if the identifier corresponds to a table alias defined in a reachable scope,
  then it is resolved to that table according to the precedence rules below.
  Further, if the table alias is followed by a `.` and a second identifier and that identifier is a column of the table, then the dotted expression resolves to
  that column.  If the table alias is not followed by a `.`, then the alias
  resolves to the entire record of each row comprising the table.
  If the identifier binds to more than one such table aliases,
  then an error is reported and the query compilation fails.

### Dynamic Table Resolution

XXX define qualified and unqualified column references

Input data data whose type is unknown (e.g., large JSON files that are not
parsed for their type information prior to compilation) is called a _dynamic table_.
Since dynamic tables have unknown types, the names of columns
is also unknown and thus the column-first resolution rules defined above
cannot be relied upon.

Moreover, the semantics of a query not depend on whether type information is known,
e.g., resolving a column or table reference one way for dynamic tables and
a different way for a typed table.
That is, a query's results
should not change merely because the input table changed from untyped to typed
for the same underlying data.

To remedy this, references of entities in dynamic tables are constrained as follows:
* A reference to a dynamic table without a column qualifer is an error;
  such references are reported and the query fails to compile.
* When there is more than one table in scope, any column reference to a dynamic
  table must include the table name in that reference; otherwise an error is
  reported and the query fails to compile.
* XXX Reference to a column where the column name is present must be
qualified either with the input table name or with `alias` to indicate
that the column alias should be used.

> [!NOTE]
> These restriction on resolving identifiers to dynamic tables
> is designed to avoid a situation where the
> semantics of a query is depedent on whether the schema is known.
> For example, the semantics of a query on dynamic tables cannot change
> if ttypeyping information is added to the dynamic table, making it a typed table,
> causing column resolution to change.  The constraints above avoid this pitfall.

## Relational Scopes

XXX Precedence of scopes.

* input scope
* output scope
* aggregate scope
* join scope
* lateral scope

XXX need to document expression mathing in agg scope e.g.,
order by J.id (see nibs)

_result set_ is the values that are produced from the output scope
or aggregate scope

peculiar in that identifiers can resolve to hidden columns
sthat do not appear in the scope's result.

## Accessing `this`

>[!NOTE]
> Diverges from relational model.
