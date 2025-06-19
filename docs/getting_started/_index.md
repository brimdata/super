---
weight: 1
title: Getting Started
breadcrumb: Introduction
---

Trying out SuperDB is easy: just [install](../getting_started/install.md) the command-line tool
[`super`](../commands/super.md) and run through its [usage documentation](../commands/super.md).

{{% tip "Note" %}}

The SuperDB code and docs are still under construction. Once you've
[installed](../getting_started/install.md) `super` we
recommend focusing first on the functionality shown in the
[`super` command doc](../commands/super.md). Feel free to explore other docs and
try things out, but please don't be shocked if you hit speedbumps in the near
term, particularly in areas like performance and full SQL coverage. We're
working on it! 😉

Once you've tried it out, we'd love to hear your feedback via
our [community Slack](https://www.brimdata.io/join-slack/).

{{% /tip %}}

Compared to putting JSON data in a relational column, the
[super-structured data model](../formats/data-model.md) makes it really easy to
mash up JSON with your relational tables.  The `super` command is a little
like [DuckDB](https://duckdb.org/) and a little like
[`jq`](https://stedolan.github.io/jq/) but super-structured data ties the
two patterns together with strong typing of dynamic values.

For a non-technical user, SuperDB is as easy to use as web search
while for a technical user, SuperDB exposes its technical underpinnings
in a gradual slope, providing as much detail as desired,
packaged up in the easy-to-understand
[Super (SUP) data format](../formats/sup.md) and
[SuperSQL language](../language/_index.md).

While `super` and its accompanying data formats are production quality for some use cases, the project's
[SuperDB data lake](../commands/super-db.md) is a bit earlier in development.

## Terminology

"Super" is an umbrella term that describes
a number of different elements of the system:
* The [super data model](../formats/data-model.md) is the abstract definition of the data types and semantics
that underlie the super-structured data formats.
* The [super-structured data formats](../formats/_index.md) are a family of
[human-readable (Super, SUP)](../formats/sup.md),
[sequential (Super Binary, BSUP)](../formats/bsup.md), and
[columnar (Super Columnar, CSUP)](../formats/csup.md) formats that all adhere to the
same abstract super data model.
* [SuperSQL](../language/_index.md) is the system's language for performing
queries, searches, analytics, transformations, or any of the above combined together.
* A [SuperSQL pipe query (SPQ)](../language/overview.md) is a query that
employs SuperSQL's unique pipeline extensions and shortcuts to perform data
operations that are difficult or impossible in standard SQL.
* A [SuperDB data lake](../commands/super-db.md) is a collection of super-structured data stored
across one or more [data pools](../commands/super-db.md#data-pools) with ACID commit semantics and
accessed via a [Git](https://git-scm.com/)-like API.

## Digging Deeper

The [SuperSQL language documentation](../language/_index.md)
is the best way to learn about `super` in depth. Most
[examples](../commands/super.md#examples) that appear throughout the docs can be
executed right in your browser and can easily be copied to the command line
for execution with `super`. Run `super -h` for a list of command options and
brief help.

The [`super db` documentation](../commands/super-db.md)
is the best way to learn about the SuperDB data lake.
All of its examples use `super db` commands run on the command line.
Run `super db -h` or `-h` with any subcommand for a list of command options
and online help.  The same language query that works for `super` operating
on local files or streams also works for `super db` operating on a lake.

## Design Philosophy

The design philosophy for SuperDB is based on composable building blocks
built from self-describing data structures.  Everything in a SuperDB data lake
is built from super-structured data and each system component can be run and tested in isolation.

Since super-structured data is self-describing, this approach makes stream composition
very easy.  Data from a query can trivially be piped to a local
instance of `super` by feeding the resulting output stream to stdin of `super`, for example,
```
super db -c "from pool | ...remote query..." | super -c "...local query..." -
```
There is no need to configure the SuperDB entities with schema information
like [protobuf configs](https://developers.google.com/protocol-buffers/docs/proto3)
or connections to
[schema registries](https://docs.confluent.io/platform/current/schema-registry/index.html).

A SuperDB data lake is completely self-contained, requiring no auxiliary databases
(like the [Hive metastore](https://hive.apache.org/development/gettingstarted))
or other third-party services to interpret the lake data.
Once copied, a new service can be instantiated by pointing a `super db serve`
at the copy of the lake.

Functionality like [data compaction](../commands/super-db.md#manage) and retention are all API-driven.

Bite-sized components are unified by the super-structured data, usually in the BSUP format:
* All lake meta-data is available via meta-queries.
* All lake operations available through the service API are also available
directly via the `super db` command.
* Lake management is agent-driven through the API.  For example, instead of complex policies
like data compaction being implemented in the core with some fixed set of
algorithms and policies, an agent can simply hit the API to obtain the meta-data
of the objects in the lake, analyze the objects (e.g., looking for too much
key space overlap) and issue API commands to merge overlapping objects
and delete the old fragmented objects, all with the transactional consistency
of the commit log.
* Components are easily tested and debugged in isolation.
