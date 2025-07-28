## SuperSQL

SuperSQL is a
[Pipe SQL](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/)
adapted for
[super-structured data](../formats/model.md).
The language is a superset of SQL query syntax and is
thus a good fit for analytics use cases
but also blends search-style pipeline shortcuts so it also works well
for interactive data exploration, search, and data wrangling.

Like a Unix pipeline, a SuperSQL query is expressed as a data source followed
by a number of commands:
```
from source | operator | operator | ...
```
As with SQL, SuperSQL is
[declarative](https://en.wikipedia.org/wiki/Declarative_programming)
and the SuperSQL compiler often optimizes a query into an implemention
different from the dataflow implied by the pipeline to achieve the 
same semantics with better performance.

### SQL Compatibility

SuperSQL is [backward compatible](../intro.md#supersql)
with relational SQL in that any SQL query is also a SuperSQL query.
SQL queries appear as a [pipe operator](operators/intro.md)
anywhere in a SuperSQL pipe query.

In particular, a single pipe operator that happens to be a SQL query
is also a SuperSQL pipe query.
For example, these are all valid SuperSQL queries:
```
SELECT 'hello, world'
SELECT * FROM table
SELECT * FROM f1.json JOIN f2.json ON f1.id=f2.id
SELECT watchers FROM https://api.github.com/repos/brimdata/super
```

### Pipe Queries

The entities that transform data within a SuperSQL pipeline are called
[pipe operators](operators/intro.md) 
and take super-structured input from the upstream operator or data source,
operate upon the input, and produce zero or more super-structured
values as output.

Unlike relational SQL, SuperSQL pipelines define their computation in terms of dataflow
through the directed graph of operators.  But instead of relational tables
propagating from one pipe operator to another
(e.g., as in [Zeta pipe SQL](https://github.com/google/zetasql/blob/master/docs/pipe-syntax.md#pipe-operator-semantics)), any sequence of potentially heterogeneously typed data
may flow between SuperSQL pipe operators.

When a super-structured sequence conforms to a single, homoegeneous
[record type](types/record.md),
then the data is equivalent to a SQL relation.
And because [any SQL query is also a valid pipe operator](sql/intro.md),
SuperSQL is thus a superset of SQL.
In particular, a single operator defined as pure SQL is an
acceptable SuperSQL query so all SQL query texts are also SuperSQL queries.

Operators like
[sort](operators/sort.md),
[aggregate](operators/aggregate.md),
[fuse](operators/fuse.md), etc. are blocking in that they
consume all of their input before produce output.

Non-blocking operators like
[where](operators/where.md),
[values](operators/values.md),
[drop](operators/drop.md), etc. produce output
incrementally while their input is consumed.

Unlike a Unix pipeline, a SuperSQL query can be forked and joined, e.g.,
```
from source
| operator
| fork
  ( operator | ... )
  ( operator | ... )
| join on condition
| ...
| switch expr
  case value ( operator | ... )
  case value ( operator | ... )
  default ( operator | ... )
| ...
```
A query can also include multiple data sources, e.g.,
```
fork
  ( from source1 | ... )
  ( from source2 | ... )
| ...
```
Here, parallel branches can be [combined](operators/combine.md) (in an undefined order),
[merged](operators/merge.md) (in a defined order) by one or more sort keys,
or [joined](operators/join.md) using relational-style join logic.

### Pipe Sources

Like SQL, input data for a query is typically sourced with the 
[`from` operator](operators/from.md).

When `from` is not present, the file arguments to the
[`super`](../commands/super.md) command are used as input to the query
as if there is an implied `from` operator, e.g., 
```
super -c "op1 | op2 | ..." input.json
```
is equivalent to 
```
super -c "from input.json | op1 | op2 | ..."
```
When neither `from` nor file arguments are specified, a single `null` value 
is provided as input to the query.
```
super -c "pass"
```
results in
```
null
```
This pattern provides a simple means to produce a constant input within a
query using the [values](operators/values.md) operator, wherein
`values` takes as input a single null and produces each constant
expression in turn, e.g.,
```
super -c "values 1,2,3"
```
results in
```
1
2
3
```

When running on the local file system, `from` may refer to a file, an HTTP
endpoint, or an [S3](../integrations/amazon-s3.md) URI.
When connected to [SuperDB database](../commands/super-db.md), `from` typically
refers to a collection of super-structured data called a "data pool" and
is referenced using the pool's name similar to SQL referencing
a relational table by name.

For more detail, see the reference page of the [`from` operator](operators/from.md),
but as an example, you might use its `from` to fetch data from an
HTTP endpoint and process it with `super`, in this case, to extract the description
and license of a GitHub repository:
```
super -f line -c """
from https://api.github.com/repos/brimdata/super
| values description,license.name
"""
```

### Dataflow Scoping

XXX intro dataflow vs relational scoping

In SQL expressions, data from tables is generally referenced with expressions that
specify a table name and a column name withing that table,
e.g., referencing a column `x` in a table `this` as
```
SELECT this.x FROM (VALUES (1),(2),(3)) AS this(x)
```
Altnernatively, when the column name is unambiguous, the table name 
can be ommitted as in
```
SELECT x FROM (VALUES (1),(2),(3)) AS this(x)
```
When SQL queries are nested, joined, or invoked as subqueries, the scoping
rules are fairly complicated and often counterintuitive.  To support such 
semantics, SuperSQL implements SQL scoping of table and column names 
_inside of of any SQL pipe operator_ but not between pipe operators.

Instead, super-structured data is referenced within a non-SQL pipe operator
using a very simple pattern:
* all input is referenced as a single value called `this`, and
* all output is emitted into a single value called `this`.

When the input to a pipe operator
is a set of homogeneously-typed [records](types/record.md), then
that data models a relational table where the record type resembles a
relational schema and each field in the record models the table's column.
In other words, the record fields of `this` can be accessed with the dot operator
reminiscent of a `table.column` reference in SQL.

For example. ,the SQL query from above can thus be written in pipe form 
using the [values operator](operators/values.md) as:
```
values {x:1}, {x:2}, {x:3} | select this.x
```
which results in:
```
{x:1}
{x:2}
{x:3}
```
As with SQL table names, `this` is optional can be omitted, i.e.,
```
values {x:1}, {x:2}, {x:3} | select x
```
produces the same result.

### Strong Typing

Data in SuperSQL is always strongly typed.

Like retional SQL, SuperSQL data sequences
can conform to a static schema that is type-checked at compile time.
And like
[document databases](https://en.wikipedia.org/wiki/Document-oriented_database)
and [SQL++](https://asterixdb.apache.org/files/SQL_Book.pdf),
data sequences may also be dynamically typed, but unlike these systems,
SuperSQL data is always strongly typed.

For example, this query produces the expected output
```
$ super -c "select b from (values (1,2),(3,4)) as T(b,c)"
{b:1}
{b:2}
```
But this query produced a compile-time error:
```
$ super -c "select a from (values (1,2),(3,4)) as T(b,c)"
column "a": does not exist at line 1, column 8:
select a from (values (1,2),(3,4)) as T(b,c)
       ~
```
Now supposing this data is in the file `input.json`:
```
{"b":1,"c":2}
{"b":3,"c":4}
```
Then, the query from above would run but because there is no predefined
schema for the sequence of JSON, "missing" values are produced:
```
$ super -c "select a from input.json"
{a:error("missing")}
{a:error("missing")}
```
While this data is strongly typed, it is also dynamically typed so that
the types are not necessarily all known ahead of time, which sometimes precludes
compile-time type checking.

That is, even though the reference to column "a" is dynamically evaluated, all
the data is still strongly typed, i.e.,
 ```
$ super -c "from input.json | values typeof(this)"
<{b:int64,c:int64}>
<{b:int64,c:int64}>
```
> In a future version of the SuperSQL compiler, we will use type information
> from SuperSQL data formats to do compile-time type checking like SQL does
> with schemas.

### Data Order

XXX

XXX explain merge/combine semantics since we took them out of operators docs.
This goes under the explanation of order, which diverges from but is backward
compatible with SQL.  Some of this is already explain in from.md

### Data Types

XXX

### Search

In addition to complex pipes and arbitrary SQL queries,
SuperSQL includes a simple [syntax for search](search.md).
Similar to an email or Web search, a simple keyword search is just a `?` followed
by the word itself, e.g.,
```
from source
| ? example.com
```
is a search for the string "example.com" and
```
from source
| ? example.com urgent
```
is a search for values with both the strings "example.com" and "urgent" present.

Unlike typical log search systems, the SuperSQL search operator is uniform:
you can specify keyword search terms mixed with Boolean predicates at any 
point in the pipe.
For example,
the predicate `message_length > 100` can simply be tacked onto the keyword search
from above, e.g.,
```
from source
| ? example.com urgent message_length > 100
```
finds all values containing the string "example.com" and "urgent" somewhere in them
provided further that the field `message_length` is a numeric value greater than 100.
A related query that performs an aggregation could be more formally
written as follows:
```
from source
| search "example.com" AND "urgent"
| where message_length > 100
| aggregate kinds:=union(type) by net:=network_of(srcip)
```
which computes an aggregation table of different message types (e.g.,
from a hypothetical field called `type`) into a new, aggregated field
called `kinds` and grouped by the network of all the source IP addresses
in the input
(e.g., from a hypothetical field called `srcip`) as a derived field called `net`.

### SQL in Pipes and Pipes in SQL

A long term goal of SuperSQL is to support a full dialect of SQL where `from` and 
`select` queries appear as any SuperSQL pipe operator.


For example, the results of the search above could feed a SQL query instead of
two pipe operators as in:
```
from source
| search "example.com" AND "urgent"
| SELECT union(type) as kinds, network_of(srcip) as net
  WHERE message_length > 100
  GROUP BY net
```
Pipe queries can also be embedded within SQL, e.g., the above query 
can be recast as
```
SELECT union(type) as kinds, network_of(srcip) as net
FROM ( from source | ? "example.com" AND "urgent")
WHERE message_length > 100
GROUP BY net
```

XXX scoping and binding / Table aliase

### Shortcuts

When exploring data interactively, it is usually much more 
ergonomic to use pipe syntax with SuperSQL's syntactic
[shortcuts](shortcuts.md).
In this way, the query from above can be written:
```
from source | ? example.com urgent message_length > 100 | kinds:=union(type) by net:=network_of(srcip)
```


## What's Next?

The following sections continue describing the Zed language.

* [The Pipeline Model](pipeline-model.md)
* [Data Types](data-types.md)
* [Const, Func, Operator, and Type Statements](statements.md)
* [Expressions](expressions.md)
* [Search Expressions](search-expressions.md)
* [Lateral Subqueries](lateral-subqueries.md)
* [Shaping and Type Fusion](shaping.md)

You may also be interested in the detailed reference materials on [operators](operators/_index.md), [functions](functions/_index.md), and [aggregate functions](aggregates/_index.md), as well as the [conventions](conventions.md) for how they're described.

