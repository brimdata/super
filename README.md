# SuperDB [![Tests][tests-img]][tests] [![GoPkg][gopkg-img]][gopkg]

SuperDB is a new analytics database that supports relational tables and JSON 
on an equal footing.  It shines when it comes to complex data wrangling use cases
where you need to explore or process large eclectic data sets.  It's also pretty
decent at analytics and
[search use cases](https://zed.brimdata.io/docs/language/search-expressions).

Unlike other relational systems that do schema inference of JSON,
SuperDB won't fall over if you throw a bunch of eclectic JSON.
You can easily do schema inference if you want, but data is ingested
by default as strongly but dynamically typed data.

In SuperDB's SQL dialect, there are no "JSON columns" so there isn't a "relational
way to do things" and a different "JSON way to do things".  Instead of having
a relational type system for structured data and completely separate JSON type
system for semi-structured data,
all data handled by SuperDB (e.g., JSON, CSV, Parquet files, Arrow streams, relational tables, etc) is automatically massaged into
[super-structured data](https://zed.brimdata.io/docs/formats/#2-zed-a-super-structured-pattern)
form.  This super-structured data is then processed by a runtime that simultaneously
supports the statically-typed relational model and the dynamically-typed 
JSON data model in a unified compute engine.

## Super JSON

Super-structured data is strongly typed and "polymorphic": any value can take on any type 
and sequences of data need not all conform to a predefined schema.  To this end,
SuperDB extends the JSON format to support super-structured data in a format called
[Super JSON](https://zed.brimdata.io/docs/formats/zson) where all JSON values 
are also Super JSON values.  Similarly,
the [Super Binary](https://zed.brimdata.io/docs/formats/zson) format is an efficient
binary representation of Super JSON (a bit like Avro) and the
[Super Columnar](https://zed.brimdata.io/docs/formats/zson) format is a columnar
representation of Super JSON (a bit like Parquet).

Even though SuperDB is based on these super-structured data formats, it can read and write
any common data format.

## Try It

Trying out SuperDB is super easy: just [install](https://zed.brimdata.io/docs/#getting-started)
the command-line tool [`super`](https://zed.brimdata.io/docs/commands/zq/).

Detailed documentation for the entire SuperDB system and its piped SQL syntax
is available on the [SuperDB docs site](https://zed.brimdata.io/docs).

The SuperDB query engine can run locally without a storage engine by accessing
files, HTTP endpoints, or S3 paths using the `super query` subcommand. While [earlier
in its development](https://zed.brimdata.io/docs/commands/zed/#status), SuperDB can also run
on a [super-structured data lake]https://zed.brimdata.io/docs/commands/zed/#1-the-lake-model)
using the `suber db ...` set of commands.

## Piped Query Syntax

The goal for SuperDB's SQL syntax (SuperSQL) is to be Postgres-compatibe and interoperate 
with BI tools though this is currently a roadmap item.  At the same time, the project
seeks to forge now ground on the usability of SQL for data exploration.  To this end,
SuperSQL supports the
[pipe query syntax](https://github.com/google/zetasql/blob/master/docs/pipe-syntax.md)
of GoogleSQL, recently described in their
[VLDB 2024 paper](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/).

In addition to the GoogleSQL syntax, SuperSQL includes additional pipeline 
operators to enhance usuability, e.g., for search, for traversing 
highly nested JSON, for data shaping, etc.

To facilitate real-time, data exploration use cases,
SuperDB supports an abbreviated form of SuperSQL called the
[SuperPipe]((https://zed.brimdata.io/docs/language) query language.
SuperPipe provides a large number of shortcuts when typing interactive 
queries, e.g., implied group-by clauses, dropping keywords,
implied keyword searches, and so forth.  Even though SuperPipe is 
a form SuperSQL, it sort of looks like the pipeline-style search languages
utilized in search systems.

### SuperDB Desktop - Coming Soon

[SuperDB Desktop](https://github.com/brimdata/zui) is an Electron-based
desktop app to explore, query, and shape data in SuperDB data lake.
It combines a search experience with a SQL query and has some really slick
design for dealing with complex and large JSON data.

Unlike most JSON browsing tools, it won't fall over if you load up ginormous
JSON values.

## Contributing

See the [contributing guide](CONTRIBUTING.md) on how you can help improve Zed!

## Join the Community

Join our [public Slack](https://www.brimdata.io/join-slack/) workspace for announcements, Q&A, and to trade tips!

[tests-img]: https://github.com/brimdata/super/workflows/Tests/badge.svg
[tests]: https://github.com/brimdata/super/actions?query=workflow%3ATests
[gopkg-img]: https://pkg.go.dev/badge/github.com/brimdata/super
[gopkg]: https://pkg.go.dev/github.com/brimdata/super

