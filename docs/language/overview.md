---
weight: 1
title: Overview
heading: Zed Language Overview
---

The Zed language is a query language for search, analytics,
and transformation inspired by the
[pipeline pattern](https://en.wikipedia.org/wiki/Tacit_programming)
of the traditional Unix shell.
Like a Unix pipeline, a query is expressed as a data source followed
by a number of commands:
```
command | command | command | ...
```
However, in Zed, the entities that transform data are called
"[operators](operators/_index.md)" instead of "commands" and unlike Unix pipelines,
the streams of data in a Zed query
are typed data sequences that adhere to the
[Zed data model](../formats/data-model.md).
Moreover, Zed sequences can be forked and joined:
```
operator
| operator
| fork (
  => operator | ...
  => operator | ...
)
| join | ...
```
Here, Zed programs can include multiple data sources and splitting operations
where multiple pipeline branches run in parallel and branches can be combined (in an
undefined order), merged (in a defined order) by one or more sort keys,
or joined using relational-style join logic.

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

## Comments

To further ease the maintenance and readability of source files, comments
beginning with `//` may appear in Zed.

```
// This includes a search with boolean logic, an expression, and an aggregation.

search "example.com" AND "urgent"
| where message_length > 100       // We only care about long messages
| aggregate kinds:=union(type) by net:=network_of(srcip)
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
