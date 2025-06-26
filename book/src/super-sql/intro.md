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

### Pipe Operators

The entities that transform data within a pipeline are called
[pipe operators](operators/intro.md) 
and take super-structured input from the upstream operator or data source,
operate upon the input, and produce zero or more super-structured
values as output.

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

Unlike a Unix pipeline, a SuperSQL query can be forked and joined:
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
A query can also include multiple data sources:
```
fork
  ( from source1 | ... )
  ( from source2 | ... )
| ...
```
Here, parallel branches can be combined (in an undefined order),
merged (in a defined order) by one or more sort keys,
or joined using relational-style join logic.

Unlike SQL, SuperSQL pipelines define their computation in terms of dataflow
through the directed graph of operators.  However, SuperSQL is a superset of SQL
in that any operator can be a SQL query. In particular, a single operator defined
as pure SQL is an acceptable SuperSQL query.

When a super-structured sequence conforms to a single, homoegeneous
[record type](types/record.md),
then the data is equivalent to a SQL relation.  Furthermore, any 
SQL relation can be represented by a record type
providing complete compatibility with relational SQL.

As with SQL, SuperSQL is
[declarative](https://en.wikipedia.org/wiki/Declarative_programming)
and the SuperSQL compiler often optimizes a query into an implemention
different from the dataflow implied by the pipeline to achieve the 
same semantics with better performance.

### Scoping and This

XXX

### Data Types

XXX

### Search

In addition to complex pipelines and arbitrary SQL queries,
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
point in the pipeline.
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

### SQL

A long term goal of SuperSQL is to support a full dialect of SQL where `from` and 
`select` queries appear as any SuperSQL pipe operator.

> XXX cite areas of SQL that are TBD

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

### Strong Typing

Data in SuperSQL is always strongly typed.  Like SQL, SuperSQL data sequences
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
Then, the query from above would run but produce "missing" results:
```
$ super -c "select a from input.json"
{a:error("missing")}
{a:error("missing")}
```
Even though the reference to column "a" is dynamically evaluated, all
the data is strongly typed, i.e.,
 ```
$ super -c "from input.json | values typeof(this)"
<{b:int64,c:int64}>
<{b:int64,c:int64}>
```
> In a future version of the SuperSQL compiler, we will use type information
> from SuperSQL data formats to do compile-time type checking like SQL does
> with schemas.

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

