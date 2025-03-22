# SuperDB [![Tests][tests-img]][tests] [![GoPkg][gopkg-img]][gopkg]

> 🔴 **NOTICE OF PROJECT READINESS** 🔴
>
> SuperDB is still under development so there's not yet a GA release.
> You're welcome to [try it out](#try-it) in its early form (i.e.,
> tip-of-`main`) and we'd love to hear your feedback. Read on for more info!

SuperDB is a new analytics database that supports relational tables and JSON
on an equal footing.  It shines when it comes to data wrangling where
you need to explore or process large eclectic data sets.  It's also pretty
decent at analytics and
[search use cases](https://superdb.org/docs/language/search-expressions).

Unlike other relational systems that do performance-fragile "schema inference" of JSON,
SuperDB won't fall over if you throw a bunch of eclectic JSON at it.
You can easily do
[schema inference if you want](https://superdb.org/docs/language/operators/fuse),
but data is ingested by default in its natural form no matter how much heterogeneity
it might have.  And unlike systems based on the document data model,
every value in SuperDB is strongly and dynamically typed thus providing the
best of both worlds: the flexibility of the document model and
the efficiency and performance of the relational model.

In SuperDB's SQL dialect, there are no "JSON columns" so there isn't a "relational
way to do things" and a different "JSON way to do things".  Instead of having
a relational type system for structured data and completely separate JSON type
system for semi-structured data,
all data handled by SuperDB (e.g., JSON, CSV, Parquet files, Arrow streams, relational tables, etc) is automatically massaged into
[super-structured data](https://superdb.org/docs/formats/#2-a-super-structured-pattern)
form.  This super-structured data is then processed by a runtime that simultaneously
supports the statically-typed relational model and the dynamically-typed
JSON data model in a unified compute engine.

## SuperSQL

SuperDB uses SQL as its query language, but it's a SQL that has been extended
with [pipe syntax](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/)
and [lots of fun shortcuts](https://superdb.org/docs/language/pipeline-model/#implied-operators).
This extended SQL is called SuperSQL.

Here's a SuperSQL query that fetches some data from GitHub Archive,
computes the set of repos touched by each user, ranks them by number of repos,
picks the top five, and joins each user with their original `created_at` time
from the current GitHub API:

```sql
FROM 'https://data.gharchive.org/2015-01-01-15.json.gz'
| SELECT union(repo.name) AS repos, actor.login AS user
  GROUP BY user
  ORDER BY len(repos) DESC
  LIMIT 5
| FORK (
  => FROM eval(f'https://api.github.com/users/{user}')
   | SELECT VALUE {user:login,created_at:time(created_at)}
  => PASS
  )
| JOIN USING (user) repos
```

## Super JSON

Super-structured data is strongly typed and "polymorphic": any value can take on any type
and sequences of data need not all conform to a predefined schema.  To this end,
SuperDB extends the JSON format to support super-structured data in a format called
[Super JSON](https://superdb.org/docs/formats/jsup) where all JSON values
are also Super JSON values.  Similarly,
the [Super Binary](https://superdb.org/docs/formats/bsup) format is an efficient
binary representation of Super JSON (a bit like Avro) and the
[Super Columnar](https://superdb.org/docs/formats/csup) format is a columnar
representation of Super JSON (a bit like Parquet).

Even though SuperDB is based on these super-structured data formats, it can read and write
most common data formats.

## Project Status

Our long-term goal for SuperSQL is to be Postgres-compatible and interoperate
with existing SQL tooling. In the meantime, SuperSQL is a bit of a moving 
target and we would love [community engagement](#join-the-community) to evolve and fine tune its
syntax and semantics.

Our areas of active development include:
* the SuperSQL query language,
* the type-based query compiler and optimizer,
* fast, vectorized ingest of common file formats,
* a complete vectorized runtime, and
* a data lake based on super-structured data.

## Try It

As SuperDB is still under construction, GA releases are not yet available.
However, you can [install]https://superdb.org/docs/getting_started/install) a build of the
[`super`](https://superdb.org/docs/commands/super) command-line tool based on
code that's under active development to start tinkering. Detailed documentation
for the SuperDB system and its piped SQL syntax is available on the
[SuperDB docs site](https://superdb.org/docs).

As the code and docs are evolving, we recommend focusing first on what's in the
[`super` command doc](https://superdb.org/docs/commands/super). Feel free to
explore other docs and try things out, but please don't be shocked if you hit
speedbumps in the near term, particularly in areas like performance and full
SQL coverage. We're working on it! :wink:

Once you've tried it out, we'd love to hear your feedback via our
[community Slack](https://www.brimdata.io/join-slack/). 

>**NOTE:** The SuperDB query engine can run locally without a storage engine by accessing
>files, HTTP endpoints, or S3 paths using the `super` command. While
>[earlier in its development](https://superdb.org/docs/commands/super-db/#status),
>SuperDB can also run on a
>[super-structured data lake](https://superdb.org/docs/commands/super-db/#the-lake-model)
>using the `super db` sub-commands.

### SuperDB Desktop - Coming Soon

[SuperDB Desktop](https://github.com/brimdata/zui) is an Electron-based
desktop app to explore, query, and shape data in a SuperDB data lake.
It combines a search experience with a SQL query and has some really slick
design for dealing with complex and large JSON data.

Unlike most JSON browsing tools, it won't slow to a crawl --- or worse crash ---
if you load it up with ginormous JSON values.

## Contributing

See the [contributing guide](CONTRIBUTING.md) on how you can help improve SuperDB!

## Join the Community

Join our [public Slack](https://www.brimdata.io/join-slack/) workspace for announcements, Q&A, and to trade tips!

[tests-img]: https://github.com/brimdata/super/workflows/Tests/badge.svg
[tests]: https://github.com/brimdata/super/actions?query=workflow%3ATests
[gopkg-img]: https://pkg.go.dev/badge/github.com/brimdata/super
[gopkg]: https://pkg.go.dev/github.com/brimdata/super

