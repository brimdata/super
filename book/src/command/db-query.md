### Command

&emsp; **query** &mdash; query a database

### Synopsis

```
super db query [options] <query>
```

### Options

TODO: unify this with super command flags.

`-aggmem` maximum memory used per aggregate function value in MiB, MB, etc
`-B` allow Super Binary to be sent to a terminal output
`-bsup.compress` compress Super Binary frames
`-bsup.framethresh` minimum Super Binary frame size in uncompressed bytes
`-color` enable/disable color formatting for -S and lake text output
`-f` format for output data [arrows,bsup,csup,csv,json,lake,line,parquet,sup,table,text,tsv,zeek,zjson]
`-fusemem` maximum memory used by fuse in MiB, MB, etc
`-I` source file containing Zed query text
`-J` use formatted JSON output independent of -f option
`-j` use line-oriented JSON output independent of -f option
`-o` write data to output file
`-persist` regular expression to persist type definitions across the stream
`-pretty` tab size to pretty print JSON and Super JSON output
`-S` use formatted Super JSON output independent of -f option
`-s` use line-oriented Super JSON output independent of -f option
`-sortmem` maximum memory used by sort in MiB, MB, etc
`-split` split output into one file per data type in this directory
`-splitsize` if >0 and -split is set, split into files at least this big rather than by data type
`-stats` display search stats on stderr
`-unbuffered` disable output buffering

Additional options of the [db sub-command](db.md#options)

### Description

The `query` command runs a [SuperSQL](../super-sql/intro.md) query with data from a lake as input.
A query typically begins with a [`from` operator](../super-sql/operators/from.md)
indicating the pool and branch to use as input.

The pool/branch names are specified with `from` in the query.

As with [`super`](super.md), the default output format is SUP for
terminals and BSUP otherwise, though this can be overridden with
`-f` to specify one of the various supported output formats.

If a pool name is provided to `from` without a branch name, then branch
"main" is assumed.

This example reads every record from the full key range of the `logs` pool
and sends the results to stdout.

```
super db query 'from logs'
```

We can narrow the span of the query by specifying a filter on the database
[sort key](db.md#sort-key):
```
super db query 'from logs | ts >= 2018-03-24T17:36:30.090766Z and ts <= 2018-03-24T17:36:30.090758Z'
```
Filters on sort keys are efficiently implemented as the data is laid out
according to the sort key and seek indexes keyed by the sort key
are computed for each data object.

When querying data to the [BSUP](../formats/bsup.md) output format,
output from a pool can be easily piped to other commands like `super`, e.g.,
```
super db query -f bsup 'from logs' | super -f table -c 'count() by field' -
```
Of course, it's even more efficient to run the query inside of the pool traversal
like this:
```
super db query -f table 'from logs | count() by field'
```
By default, the `query` command scans pool data in sort-key order though
the query optimizer may, in general, reorder the scan to optimize searches,
aggregations, and joins.
An order hint can be supplied to the `query` command to indicate to
the optimizer the desired processing order, but in general,
the [sort](../super-sql/operators/sort.md) operator
should be used to guarantee any particular sort order.

Arbitrarily complex queries can be executed over the lake in this fashion
and the planner can utilize cloud resources to parallelize and scale the
query over many parallel workers that simultaneously access the lake data in
shared cloud storage (while also accessing locally- or cluster-cached copies of data).

#### Meta-queries

Commit history, metadata about data objects, database and pool configuration,
etc. can all be queried and
returned as super-structured data, which in turn, can be fed into analytics.
This allows a very powerful approach to introspecting the structure of a
lake making it easy to measure, tune, and adjust lake parameters to
optimize layout for performance.

These structures are introspected using meta-queries that simply
specify a metadata source using an extended syntax in the `from` operator.
There are three types of meta-queries:
* `from :<meta>` - lake level
* `from pool:<meta>` - pool level
* `from pool[@<branch>]<:meta>` - branch level

`<meta>` is the name of the metadata being queried. The available metadata
sources vary based on level.

For example, a list of pools with configuration data can be obtained
in the SUP format as follows:
```
super db query -S "from :pools"
```
This meta-query produces a list of branches in a pool called `logs`:
```
super db query -S "from logs:branches"
```
You can filter the results just like any query,
e.g., to look for particular branch:
```
super db query -S "from logs:branches | branch.name=='main'"
```

This meta-query produces a list of the data objects in the `live` branch
of pool `logs`:
```
super db query -S "from logs@live:objects"
```

You can also pretty-print in human-readable form most of the metadata records
using the "lake" format, e.g.,
```
super db query -f lake "from logs@live:objects"
```

The `main` branch is queried by default if an explicit branch is not specified,
e.g.,

```
super db query -f lake "from logs:objects"
```
