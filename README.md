# SuperDB [![Tests][tests-img]][tests] [![GoPkg][gopkg-img]][gopkg]

SuperDB is an analytics database that supports relational tables and JSON 
on an equal footing.  It shines when it comes to complex data wrangling use cases
where you need to explore large eclectic data sets.  It's also pretty decent
at analytics and search use cases.

In SuperDB's SQL dialect, there are no "JSON columns" so there isn't a "relational
way to do things" and a different "JSON way to do things".  Instead of having
a relational type system for structured data and completely separate JSON type
system for semi-structured data,
all data handled by SuperDB (e.g., JSON, CSV, Parquet files, Arrow streams, relational tables, etc) is automatically massaged into
[super-structured data](https://zed.brimdata.io/docs/formats/#2-zed-a-super-structured-pattern)
form.  This super-structured data is then processed by a runtime that simultaneously
supports the statically-typed relational model and the dynamically-typed 
JSON data model in a unified compute engine.

Super-structured data is strongly typed and "polymorphic": any value can take on any type 
and sequences of data need not all conform to a predefined schema.  To this end,
SuperDB extends the JSON format to support super-structured data in a format called
[Super JSON](https://zed.brimdata.io/docs/formats/zson) where all JSON possible values 
are also Super JSON values.  Similarly,
the [Super Binary](https://zed.brimdata.io/docs/formats/zson) format is an efficient
binary representation of Super JSON (a bit like Avro) and the
[Super Columnar](https://zed.brimdata.io/docs/formats/zson) format is a columnar
representation of Super JSON (a bit like Parquet).

Even though SuperDB is based on these super-structured data formats, it can read and write
any common data format.

Trying out SuperDB is super easy: just [install](https://zed.brimdata.io/docs/#getting-started)
the command-line tool [`super`](https://zed.brimdata.io/docs/commands/zq/).

The SuperDB query engine can run locally without a storage engine by accessing
files, HTTP endpoints, or S3 paths using the `super query` subcommand. While [earlier
in its development](https://zed.brimdata.io/docs/commands/zed/#status), SuperDB can also run
on a [super-structured data lake]https://zed.brimdata.io/docs/commands/zed/#1-the-lake-model)
using the `suber db ...` set of commands.

## Pipe Query Syntax

The goal for SuperDB's SQL syntax (SuperSQL) is to be Postgres-compatibe and interoperate 
with BI tools though this is currently a roadmap item.  At the same time, the project
seeks to forge now ground on the usability of SQL for data exploration.  To this end,
SuperSQL supports the
[pipe query syntax](https://github.com/google/zetasql/blob/master/docs/pipe-syntax.md)
of GoogleSQL, recently described in their
[VLDB 2024 paper](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/).

In addition to the GoogleSQL syntax, SuperSQL includes additional pipeline 
operators to enhance usuability, e.g., for search and for traversing 
highly nested JSON.

To facilitate real-time, data exploration use cases,
SuperDB supports an abbreviated form of SuperSQL called the
[SuperPipe]((https://zed.brimdata.io/docs/language) query language.
SuperPipe provides a large number of shortcuts when typing interactive 
queries, e.g., implied group-by clauses, dropping keywords,
implied keyword searches, and so forth.  Even though SuperPipe is 
a form SuperSQL, it sort of looks like the pipeline-style search languages
utilized in search systems.


XXX TODO ...

## Why?

We think data is hard and it should be much, much easier.

While _schemas_ are a great way to model and organize your data, they often
[get in the way](https://github.com/brimdata/sharkfest-21#schemas-a-double-edged-sword)
when you are just trying to store or transmit your semi-structured data.

Also, why should you have to set up one system
for search and another completely different system for historical analytics?
And the same unified search/analytics system that works at cloud scale should run easily as
a lightweight command-line tool on your laptop.

And rather than having to set up complex ETL pipelines with brittle
transformation logic, managing your data lake should be as easy as
[`git`](https://git-scm.com/).

Finally, we believe a lightweight data store that provides easy search and analytics
would be a great place to store data sets for data science and
data engineering experiments running in Python and providing easy
integration with your favorite Python libraries.

## How?

Zed solves all these problems with a new foundational data format called
[ZSON](https://zed.brimdata.io/docs/formats/zson),
which is a superset of JSON and the relational models.
ZSON is syntax-compatible with JSON
but it has a comprehensive type system that you can use as little or as much as you like.
Zed types can be used as schemas.

The [Zed language](https://zed.brimdata.io/docs/language) offers a gentle learning curve,
which spans the gamut from simple
[keyword search](https://zed.brimdata.io/docs/language/#7-search-expressions)
to powerful data-transformation operators like
[lateral sub-queries](https://zed.brimdata.io/docs/language/#8-lateral-subqueries)
and [shaping](https://zed.brimdata.io/docs/language/#9-shaping).

Zed also has a cloud-based object design that was modeled after
the `git` design pattern.  Commits to the lake are transactional
and consistent.

## Quick Start

Check out the [installation page](https://zed.brimdata.io/docs/install/)
for a quick and easy install.

Detailed documentation for the entire Zed system and language
is available on the [Zed docs site](https://zed.brimdata.io/docs).

### Zui

The [Zui app](https://github.com/brimdata/zui) is an Electron-based
desktop app to explore, query, and shape data in your Zed lake.

We originally developed Zui for security-oriented use cases
(having tight integration with [Zeek](https://zeek.org/),
[Suricata](https://suricata.io/), and
[Wireshark](https://www.wireshark.org/)),
but we are actively extending Zui with UX for handling generic
data sets to support data science, data engineering, and ETL use cases.

## Contributing

See the [contributing guide](CONTRIBUTING.md) on how you can help improve Zed!

## Join the Community

Join our [public Slack](https://www.brimdata.io/join-slack/) workspace for announcements, Q&A, and to trade tips!

## Acknowledgment

We modeled this README after
Philip O'Toole's brilliantly succinct
[description of `rqlite`](https://github.com/rqlite/rqlite).

[tests-img]: https://github.com/brimdata/super/workflows/Tests/badge.svg
[tests]: https://github.com/brimdata/super/actions?query=workflow%3ATests
[gopkg-img]: https://pkg.go.dev/badge/github.com/brimdata/super
[gopkg]: https://pkg.go.dev/github.com/brimdata/super
