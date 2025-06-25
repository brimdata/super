# SuperSQL

XXX data order... goes in super-sql inrto

XXX talk about sum types ... missing from Codd's footnote.  achilles heel of modern SQL systems.  even systems that have it make it cumberson with type discriminator or 
named descrimintor
XXX ex with duckdb where you can't read JSON into union columns without the 
named subscripts

TODO...

SuperSQL is a Pipe SQL adapted for super-structured data.
It is a superset of SQL and is thus a good fit for analytics use cases
but also blends search-style pipeline shortcuts so it also works well
for interactive data exploration, search, and data wrangling.

Like a Unix pipeline, a query is expressed as a data source followed
by a number of commands:
```
from source | command | command | ...
```
The entities that transform data within a pipeline are called
[operators](operators/_index.md) 
are typed data sequences that adhere to the
[Zed data model](../formats/data-model.md).
Moreover, Zed sequences can be forked and joined:
```
from source
| operator
| fork (
  ( operator | ... )
  ( operator | ... )
| join | ...
| switch expr
  case value ( operator | ... )
  case value ( operator | ... )
  default ( operator | ... )
| ...
```
Here, a SuperQL query can include multiple data sources and splitting operations
where multiple pipeline branches run in parallel and branches can be combined (in an
undefined order), merged (in a defined order) by one or more sort keys,
or joined using relational-style join logic.

Unlike SQL, SuperSQL pipelines define their computation in terms of dataflow
through the directed graph of operators.  However, SuperSQL is a superset of SQL
in that any operator can be a SQL query. In particular, a single operator defined
as pure SQL is an acceptable SuperSQL query.

> XXX cite areas of SQL that are TBD

Generally speaking, a [flow graph](https://en.wikipedia.org/wiki/Directed_acyclic_graph)
defines a directed acyclic graph (DAG) composed
of data sources and operator nodes.  The Zed syntax leverages "fat arrows",
i.e., `=>`, to indicate the start of a parallel branch of the pipeline.

That said, the Zed language is
[declarative](https://en.wikipedia.org/wiki/Declarative_programming)
and the Zed compiler optimizes the pipeline computation
&mdash; e.g., often implementing a Zed program differently than
the flow implied by the pipeline yet reaching the same result &mdash;
much as a modern SQL engine optimizes a declarative SQL query.

## Search and Analytics

Zed is also intended to provide a seamless transition from a simple search experience
(e.g., typed into a search bar or as the query argument of the [`super`](../commands/super.md) command-line
tool) to more a complex analytics experience composed of complex joins and aggregations
where the Zed language source text would typically be authored in a editor and
managed under source-code control.

Like an email or Web search, a simple keyword search is just the word itself,
e.g.,
```
example.com
```
is a search for the string "example.com" and
```
example.com urgent
```
is a search for values with both the strings "example.com" and "urgent" present.

Unlike typical log search systems, the Zed language operators are uniform:
you can specify an operator including keyword search terms, Boolean predicates,
etc. using the same [search expression](search-expressions.md) syntax at any point
in the pipeline.

For example,
the predicate `message_length > 100` can simply be tacked onto the keyword search
from above, e.g.,
```
example.com urgent message_length > 100
```
finds all values containing the string "example.com" and "urgent" somewhere in them
provided further that the field `message_length` is a numeric value greater than 100.
A related query that performs an aggregation could be more formally
written as follows:
```
search "example.com" AND "urgent"
| where message_length > 100
| aggregate kinds:=union(type) by net:=network_of(srcip)
```
which computes an aggregation table of different message types (e.g.,
from a hypothetical field called `type`) into a new, aggregated field
called `kinds` and grouped by the network of all the source IP addresses
in the input
(e.g., from a hypothetical field called `srcip`) as a derived field called `net`.

The short-hand query from above might be typed into a search box while the
latter query might be composed in a query editor or in Zed source files
maintained in GitHub.  Both forms are valid Zed queries.

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

