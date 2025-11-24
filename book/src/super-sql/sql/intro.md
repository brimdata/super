## SQL Operators

SuperSQL is backward compatible with SQL in that any SQL query
is a SuperSQL [pipe operator](../operators/intro.md).
A SQL query used as a pipe operator in SuperSQL is called a _SQL operator_.

A SQL query typically specifies its input using one or more
[FROM](from.md) clauses, but when the FROM clause is omitted,
the query takes its input from the parent operator.
If there is no parent operator and `FROM` is omitted, then the
default input is a single `null` value.

### Operator Structure

A SQL operator is a query having the form of a `<query-envelope>` defined as
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

The `<query-body>` has one of the following forms:
* a [SELECT](select.md) clause,
* a [VALUES](values.md) clause,
* a [set operation](#set-operators), or
* a parenthesized query of the form `( <query-envelope> )`.

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

### Set Operators

SQL set operators combine two input relations using set union, set intersection,
and set substraction.

A set operation has the form:
```
<query-envelope> UNION [ALL | DISTINCT] <query-envelope>
<query-envelope> INTERSECT [ALL | DISTINCT] <query-envelope>
<query-envelope> EXCLUDE [ALL | DISTINCT] <query-envelope>
```
where `<query-envelope>` is a query as [defined above](#operator-structure).

These binary operators are left associative.  Parenthesized queries
may be used to override the default left-to-right evaluation order.

Currently, only the [UNION](union.md) set operator is supported.

>[!NOTE]
> The `INTERSECT` AND `EXCLUDE` operators are not yet available in SuperSQL.

### Identifier Resolution

Identifiers that appear in SQL expressions are resolved in accordance
with the relational model.  Each SQL operator defines a relational namespace
that is independent of other SQL operators and does not span across
pipe operator boundaries.

A [FROM](from.md) clause creates this namespace, which defines one or more
table names each containing one or more column names.

A particular column is referenced by name using the syntax
```
<column>
```
or
```
<table> . <column>
```
where `<table>` and `<column>` are identifiers.

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

Identifiers that correspond to non-column references have precedence
over columns and are resolved as follows:


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
  provided that the identifier resolve to no other columns in other reacjable tables;
  if multiple tables match, then an error is reported and the query fails to compile.
* Otherwise, if the identifier corresponds to a table alias defined in a reachable scope,
  then it is resolved to that table according to the precedence rules below.
  Futher, if the table alias is followed by a `.` and a second identifier and that identifier is a column of the table, then the dotted expression resolves to
  that column.  If the table alias is not followed by a `.`, then the alias
  resolves to the entire record of each row comprising the table.
  If the identifier binds to more than one such table aliases,
  then an error is reported and the query compilation fails.

For dynamically typed data, a reference to a table alias cannot be used by itself,
i.e., without a column reference.  In this case, an error is reported
and the query fails to compile.

> [!NOTE]
> This restriction on table aliases for dynamic data
> is designed to avoid a situation where the
> results of a query is depedent on whether the schema is known.  For dynamic data,
> whether the table alias appears as a column or not is unknown and thus resolution
> cannot be carried out.

### Relational Scopes

XXX Precedence of scopes.

* the identifier is 

TODO: section on scoping

see [issue](https://github.com/brimdata/super/issues/5974)

CTE scoping

### Input References

### Accessing `this`

>[!NOTE]
> Diverges from relational model.
