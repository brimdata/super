---
sidebar_position: 1
sidebar_label: super
---

# `super`

> **TL;DR** `super` is a command-line tool that uses [SuperSQL](../language/README.md)
> to query a variety of data formats in files, over HTTP, or in [S3](../integrations/amazon-s3.md)
> storage. Best performance is achieved when operating on data in binary formats such as
> [Super Binary](../formats/bsup.md), [Super Columnar](../formats/csup.md),
> [Parquet](https://github.com/apache/parquet-format), or
> [Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format).

## Usage

```
super [ options ] [ -c query ] input [ input ... ]
```

`super` is a command-line tool for processing data in diverse input
formats, providing data wrangling, search, analytics, and extensive transformations
using the [SuperSQL](../language/README.md) dialect of SQL. Any SQL query expression
may be extended with [pipe syntax](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/)
to filter, transform, and/or analyze input data.
Super's SQL pipes dialect is extensive, so much so that it can resemble
a log-search experience despite its SQL foundation.

The `super` command works with data from ephemeral sources like files and URLs.
If you want to persist your data into a data lake for persistent storage,
check out the [`super db`](super-db.md) set of commands.

By invoking the `-c` option, a query expressed in the [SuperSQL language](../language/README.md)
may be specified and applied to the input stream.

The [super data model](../formats/zed.md) is based on [super-structured data](../formats/README.md#2-a-super-structured-pattern), meaning that all data
is both strongly _and_ dynamically typed and need not conform to a homogeneous
schema.  The type structure is self-describing so it's easy to daisy-chain
queries and inspect data at any point in a complex query or data pipeline.
For example, there's no need for a set of Parquet input files to all be
schema-compatible and it's easy to mix and match Parquet with JSON across
queries.

When processing JSON data, all values are converted to strongly typed values
that fit naturally alongside relational data so there is no need for a separate
"JSON type".  Unlike SQL systems that integrate JSON data,
there isn't a JSON way to do things and a separate relational way
to do things.

Because there are no schemas, there is no schema inference, so inferred schemas
do not haphazardly change when input data changes in subtle ways.

Each `input` argument to `super` must be a file path, an HTTP or HTTPS URL,
an S3 URL, or standard input specified with `-`.
These input arguments are treated as if a SQL `FROM` operator precedes
the provided query, e.g.,
```
super -c "FROM example.json | SELECT typeof(this)"
```
is equivalent to
```
super -c "SELECT typeof(this)" example.json
```
and both are equivalent to the classic SQL
```
super -c "SELECT typeof(this) FROM example.json"
```
Output is written to one or more files or to standard output in the format specified.

When multiple input files are specified, they are processed in the order given as
if the data were provided by a single, concatenated `FROM` clause.

If no query is specified with `-c`, the inputs are scanned without modification
and output in the desired format as [described below](#input-formats),
providing a convenient means to convert files from one format to another, e.g.,
```
super -f arrows file1.json file2.parquet file3.csv > file-combined.arrows
```
When `super` is run with a query that has no `FROM` operator and no input arguments,
the SuperSQL query is fed a single `null` value analogous to SQL's default
input of a single empty row of an unnamed table.
This provides a convenient means to explore examples or run in a
"calculator mode", e.g.,
```mdtest-command
super -z -c '1+1'
```
emits
```mdtest-output
2
```
Note that SuperSQL's has syntactic shortcuts for interactive data exploration and
an expression that stands alone is a shortcut for `SELECT VALUE`, e.g., the query text
```
1+1
```
is equivalent to
```
SELECT VALUE 1+1
```
To learn more about shortcuts, refer to the SuperSQL
[documentation on shortcuts](../language/pipeline-model.md#implied-operators).

For built-in command help and a listing of all available options,
simply run `super` with no arguments.

## Data Formats

`super` supports a number of [input](#input-formats) and [output](#output-formats) formats, but the super formats
([Super Binary](../formats/bsup.md),
[Super Columnar](../formats/csup.md),
and [Super JSON](../formats/jsup.md)) tend to be the most versatile and
easy to work with.

`super` typically operates on binary-encoded data and when you want to inspect
human-readable bits of output, you merely format it as Super JSON, which is the
default format when output is directed to the terminal.  Super Binary is the default
when redirecting to a non-terminal output like a file or pipe.

Unless the `-i` option specifies a specific input format,
each input's format is [automatically inferred](#auto-detection)
and each input is scanned
in the order appearing on the command line forming the input stream.

### Input Formats

`super` currently supports the following input formats:

|  Option   | Auto | Specification                            |
|-----------|------|------------------------------------------|
| `arrows`  |  yes | [Arrow IPC Stream Format](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format) |
| `bsup`    |  yes | [Super Binary](../formats/bsup.md) |
| `csup`    |  yes | [Super Columnar](../formats/csup.md) |
| `csv`     |  yes | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `json`    |  yes | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
| `jsup`    |  yes | [Super JSON](../formats/jsup.md) |
| `zjson`   |  yes | [Super JSON over JSON](../formats/zjson.md) |
| `line`    |  no  | One string value per input line |
| `parquet` |  yes | [Apache Parquet](https://github.com/apache/parquet-format) |
| `tsv`     |  yes | [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values) |
| `zeek`    |  yes | [Zeek Logs](https://docs.zeek.org/en/master/logs/index.html) |

The input format is typically [detected automatically](#auto-detection) and the formats for which
"Auto" is "yes" in the table above support _auto-detection_.
Formats without auto-detection require the `-i` option.

#### Hard-wired Input Format

The input format is specified with the `-i` flag.

When `-i` is specified, all of the inputs on the command-line must be
in the indicated format.

#### Auto-detection

When using _auto-detection_, each input's format is independently determined
so it is possible to easily blend different input formats into a unified
output format.

For example, suppose this content is in a file `sample.csv`:
```mdtest-input sample.csv
a,b
1,foo
2,bar
```
and this content is in `sample.json`
```mdtest-input sample.json
{"a":3,"b":"baz"}
```
then the command
```mdtest-command
super -z sample.csv sample.json
```
would produce this output in the default Super JSON format
```mdtest-output
{a:1.,b:"foo"}
{a:2.,b:"bar"}
{a:3,b:"baz"}
```

#### JSON Auto-detection: Super vs. Plain

Since [Super JSON](../formats/jsup.md) is a superset of plain JSON, `super` must be careful how it distinguishes the two cases when performing auto-inference.
While you can always clarify your intent
via `-i jsup` or `-i json`, `super` attempts to "just do the right thing"
when you run it with Super JSON vs. plain JSON.

While `super` can parse any JSON using its built-in Super JSON parser this is typically
not desirable because (1) the Super JSON parser is not particularly performant and
(2) all JSON numbers are floating point but the Super JSON parser will parse as
JSON any number that appears without a decimal point as an integer type.

:::tip note
The reason `super` is not particularly performant for Super JSON is that the [Super Binary](../formats/bsup.md) or
[Super Columnar](../formats/csup.md) formats are semantically equivalent to Super JSON but much more efficient and
the design intent is that these efficient binary formats should be used in
use cases where performance matters.  Super JSON is typically used only when
data needs to be human-readable in interactive settings or in automated tests.
:::

To this end, `super` uses a heuristic to select between Super JSON and plain JSON when the
`-i` option is not specified. Specifically, plain JSON is selected when the first values
of the input are parsable as valid JSON and includes a JSON object either
as an outer object or as a value nested somewhere within a JSON array.

This heuristic almost always works in practice because Super JSON records
typically omit quotes around field names.

### Output Formats

`super` currently supports the following output formats:

|  Option   | Specification                            |
|-----------|------------------------------------------|
| `arrows`  | [Arrow IPC Stream Format](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format) |
| `bsup`    | [Super Binary](../formats/bsup.md) |
| `csup`    | [Super Columnar](../formats/csup.md) |
| `csv`     | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `json`    | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
| `jsup`    | [Super JSON](../formats/jsup.md) |
| `zjson`   | [Super JSON over JSON](../formats/zjson.md) |
| `lake`    | [SuperDB Data Lake Metadata Output](#superdb-data-lake-metadata-output) |
| `parquet` | [Apache Parquet](https://github.com/apache/parquet-format) |
| `table`   | (described [below](#simplified-text-outputs)) |
| `text`    | (described [below](#simplified-text-outputs)) |
| `tsv`     | [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values) |
| `zeek`    | [Zeek Logs](https://docs.zeek.org/en/master/logs/index.html) |

The output format defaults to either Super JSON or Super Binary and may be specified
with the `-f` option.

Since Super JSON is a common format choice, the `-z` flag is a shortcut for
`-f jsup`.  Also, `-Z` is a shortcut for `-f jsup` with `-pretty 4` as
[described below](#pretty-printing).

And since plain JSON is another common format choice, the `-j` flag is a shortcut for
`-f json` and `-J` is a shortcut for pretty printing JSON.

#### Output Format Selection

When the format is not specified with `-f`, it defaults to Super JSON if the output
is a terminal and to Super Binary otherwise.

While this can cause an occasional surprise (e.g., forgetting `-f` or `-z`
in a scripted test that works fine on the command line but fails in CI),
we felt that the design of having a uniform default had worse consequences:
* If the default format were Super JSON, it would be very easy to create pipelines
and deploy to production systems that were accidentally using Super JSON instead of
the much more efficient Super Binary format because the `-f bsup` had been mistakenly
omitted from some command.  The beauty of SuperDB is that all of this "just works"
but it would otherwise perform poorly.
* If the default format were Super Binary, then users would be endlessly annoyed by
binary output to their terminal when forgetting to type `-f jsup`.

In practice, we have found that the output defaults
"just do the right thing" almost all of the time.

#### Pretty Printing

Super JSON and plain JSON text may be "pretty printed" with the `-pretty` option, which takes
the number of spaces to use for indentation.  As this is a common option,
the `-Z` option is a shortcut for `-f jsup -pretty 4` and `-J` is a shortcut
for `-f json -pretty 4`.

For example,
```mdtest-command
echo '{a:{b:1,c:[1,2]},d:"foo"}' | super -Z -
```
produces
```mdtest-output
{
    a: {
        b: 1,
        c: [
            1,
            2
        ]
    },
    d: "foo"
}
```
and
```mdtest-command
echo '{a:{b:1,c:[1,2]},d:"foo"}' | super -f jsup -pretty 2 -
```
produces
```mdtest-output
{
  a: {
    b: 1,
    c: [
      1,
      2
    ]
  },
  d: "foo"
}
```

When pretty printing, colorization is enabled by default when writing to a terminal,
and can be disabled with `-color false`.

#### Pipeline-friendly Super Binary

Though it's a compressed format, Super Binary data is self-describing and stream-oriented
and thus is pipeline friendly.

Since data is self-describing you can simply take Super Binary output
of one command and pipe it to the input of another.  It doesn't matter if the value
sequence is scalars, complex types, or records.  There is no need to declare
or register schemas or "protos" with the downstream entities.

In particular, Super Binary data can simply be concatenated together, e.g.,
```mdtest-command
super -f bsup -c 'select value 1, [1,2,3]' > a.bsup
super -f bsup -c 'select value {s:"hello"}, {s:"world"}' > b.bsup
cat a.bsup b.bsup | super -z -
```
produces
```mdtest-output
1
[1,2,3]
{s:"hello"}
{s:"world"}
```
And while this Super JSON output is human readable, the Super Binary files are binary, e.g.,
```mdtest-command
super -f bsup -c 'select value 1,[ 1,2,3]' > a.bsup
hexdump -C a.bsup
```
produces
```mdtest-output
00000000  02 00 01 09 1b 00 09 02  02 1e 07 02 02 02 04 02  |................|
00000010  06 ff                                             |..|
00000012
```

#### Schema-rigid Outputs

Certain data formats like [Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format)
and [Parquet](https://github.com/apache/parquet-format) are "schema rigid" in the sense that
they require a schema to be defined before values can be written into the file
and all the values in the file must conform to this schema.

SuperDB, however, has a fine-grained type system instead of schemas such that a sequence
of data values is completely self-describing and may be heterogeneous in nature.
This creates a challenge converting the type-flexible super-structured data formats to a schema-rigid
format like Arrow and Parquet.

For example, this seemingly simple conversion:
```mdtest-command fails
echo '{x:1}{s:"hello"}' | super -o out.parquet -f parquet -
```
causes this error
```mdtest-output
parquetio: encountered multiple types (consider 'fuse'): {x:int64} and {s:string}
```

##### Fusing Schemas

As suggested by the error above, the [`fuse` operator](../language/operators/fuse.md) can merge different record
types into a blended type, e.g., here we create the file and read it back:
```mdtest-command
echo '{x:1}{s:"hello"}' | super -o out.parquet -f parquet -c fuse -
super -z out.parquet
```
but the data was necessarily changed (by inserting nulls):
```mdtest-output
{x:1,s:null(string)}
{x:null(int64),s:"hello"}
```

##### Splitting Schemas

Another common approach to dealing with the schema-rigid limitation of Arrow and
Parquet is to create a separate file for each schema.

`super` can do this too with the `-split` option, which specifies a path
to a directory for the output files.  If the path is `.`, then files
are written to the current directory.

The files are named using the `-o` option as a prefix and the suffix is
`-<n>.<ext>` where the `<ext>` is determined from the output format and
where `<n>` is a unique integer for each distinct output file.

For example, the example above would produce two output files,
which can then be read separately to reproduce the original data, e.g.,
```mdtest-command
echo '{x:1}{s:"hello"}' | super -o out -split . -f parquet -
super -z out-*.parquet
```
produces the original data
```mdtest-output
{x:1}
{s:"hello"}
```

While the `-split` option is most useful for schema-rigid formats, it can
be used with any output format.

#### Simplified Text Outputs

The `text` and `table` formats simplify data to fit within the
limitations of text-based output. Because they do not capture all the
information required to reconstruct the original data, they are not supported
input formats. They may be a good fit for use with other text-based shell
tools, but due to their limitations should be used with care.

In `text` output, minimal formatting is applied, e.g., strings are shown
without quotes and brackets are dropped from [arrays](../formats/zed.md#22-array)
and [sets](../formats/zed.md#23-set). [Records](../formats/zed.md#21-record)
are printed as tab-separated field values without their corresponding field
names. For example:

```mdtest-command
echo '"hi" {hello:"world",good:"bye"} [1,2,3]' | super -f text -
```
produces
```mdtest-output
hi
world	bye
1,2,3
```

The `table` format includes header lines showing the field names in records.
For example:

```mdtest-command
echo '{word:"one",digit:1} {word:"two",digit:2}' | super -f table -
```
produces
```mdtest-output
word digit
one  1
two  2
```

If a new record type is encountered in the input stream that does not match
the previously-printed header line, a new header line will be output.
For example:

```mdtest-command
echo '{word:"one",digit: 1} {word:"hello",style:"greeting"}' |
  super -f table -
```
produces
```mdtest-output
word digit
one  1
word  style
hello greeting
```

If this is undesirable, the [`fuse` operator](../language/operators/fuse.md)
may prove useful to unify the input stream under a single record type that can
be described with a single header line. Doing this to our last example, we find

```mdtest-command
echo '{word:"one",digit:1} {word:"hello",style:"greeting"}' |
  super -f table -c 'fuse' -
```
now produces
```mdtest-output
word  digit style
one   1     -
hello -     greeting
```

#### SuperDB Data Lake Metadata Output

The `lake` format is used to pretty-print lake metadata, such as in
[`super db` sub-command](super-db.md) outputs.  Because it's `super db`'s default output format,
it's rare to request it explicitly via `-f`.  However, since it's possible for
`super db` to [generate output in any supported format](super-db.md#super-db-commands),
the `lake` format is useful to reverse this.

For example, imagine you'd executed a [meta-query](super-db.md#meta-queries) via
`super db query -Z "from :pools"` and saved the output in this file `pools.jsup`.

```mdtest-input pools.jsup
{
    ts: 2024-07-19T19:28:22.893089Z,
    name: "MyPool",
    id: 0x132870564f00de22d252b3438c656691c87842c2 (=ksuid.KSUID),
    layout: {
        order: "desc" (=order.Which),
        keys: [
            [
                "ts"
            ] (=field.Path)
        ] (=field.List)
    } (=order.SortKey),
    seek_stride: 65536,
    threshold: 524288000
} (=pools.Config)
```

Using `super -f lake`, this can be rendered in the same pretty-printed form as it
would have originally appeared in the output of `super db ls`, e.g.,

```mdtest-command
super -f lake pools.jsup
```
produces
```mdtest-output
MyPool 2jTi7n3sfiU7qTgPTAE1nwTUJ0M key ts order desc
```

## Query Debugging

If you are ever stumped about how the `super` compiler is parsing your query,
you can always run `super -C` to compile and display your query in canonical form
without running it.
This can be especially handy when you are learning the language and
[its shortcuts](../language/pipeline-model.md#implied-operators).

For example, this query
```mdtest-command
super -C -c 'has(foo)'
```
is an implied [`where` operator](../language/operators/where.md), which matches values
that have a field `foo`, i.e.,
```mdtest-output
where has(foo)
```
while this query
```mdtest-command
super -C -c 'a:=x+1'
```
is an implied [`put` operator](../language/operators/put.md), which creates a new field `a`
with the value `x+1`, i.e.,
```mdtest-output
put a:=x+1
```

## Error Handling

Fatal errors like "file not found" or "file system full" are reported
as soon as they happen and cause the `super` process to exit.

On the other hand,
runtime errors resulting from the query itself
do not halt execution.  Instead, these error conditions produce
[first-class errors](../language/data-types.md#first-class-errors)
in the data output stream interleaved with any valid results.
Such errors are easily queried with the
[`is_error` function](../language/functions/is_error.md).

This approach provides a robust technique for debugging complex queries,
where errors can be wrapped in one another providing stack-trace-like debugging
output alongside the output data.  This approach has emerged as a more powerful
alternative to the traditional technique of looking through logs for errors
or trying to debug a halted query with a vague error message.

For example, this query
```mdtest-command
echo '1 2 0 3' | super -z -c '10.0/this' -
```
produces
```mdtest-output
10.
5.
error("divide by zero")
3.3333333333333335
```
and
```mdtest-command
echo '1 2 0 3' | super -c '10.0/this' - | super -z -c 'is_error(this)' -
```
produces just
```mdtest-output
error("divide by zero")
```

## Examples

As you may have noticed, many examples of the [SuperSQL language](../language/README.md)
are illustrated using this pattern
```
echo <values> | super -c <query> -
```
which is used throughout the [language documentation](../language/README.md)
and [operator reference](../language/operators/README.md).

The language documentation and [tutorials directory](../tutorials/README.md)
have many examples, but here are a few more simple `super` use cases.

_Hello, world_
```mdtest-command
super -z -c "SELECT VALUE 'hello, world'"
```
produces this Super JSON output
```mdtest-output
"hello, world"
```

_Some values of available [data types](../language/data-types.md)_
```mdtest-command
echo '1 1.5 [1,"foo"] |["apple","banana"]|' | super -z -
```
produces
```mdtest-output
1
1.5
[1,"foo"]
|["apple","banana"]|
```
_The types of various data_
```mdtest-command
echo '1 1.5 [1,"foo"] |["apple","banana"]|' | super -z -c 'SELECT VALUE typeof(this)' -
```
produces
```mdtest-output
<int64>
<float64>
<[(int64,string)]>
<|[string]|>
```
_A simple [aggregation](../language/aggregates/README.md)_
```mdtest-command
echo '{key:"foo",val:1}{key:"bar",val:2}{key:"foo",val:3}' |
  super -z -c 'sum(val) by key | sort key' -
```
produces
```mdtest-output
{key:"bar",sum:2}
{key:"foo",sum:4}
```
_Read CSV input and [cast](../language/functions/cast.md) a to an integer from default float_
```mdtest-command
printf "a,b\n1,foo\n2,bar\n" | super -z -c 'a:=int64(a)' -
```
produces
```mdtest-output
{a:1,b:"foo"}
{a:2,b:"bar"}
```
_Read JSON input and cast to an integer from default float_
```mdtest-command
echo '{"a":1,"b":"foo"}{"a":2,"b":"bar"}' | super -z -c 'a:=int64(a)' -
```
produces
```mdtest-output
{a:1,b:"foo"}
{a:2,b:"bar"}
```
_Make a schema-rigid Parquet file using fuse, then output the Parquet file as Super JSON_
```mdtest-command
echo '{a:1}{a:2}{b:3}' | super -f parquet -o tmp.parquet -c fuse -
super -z tmp.parquet
```
produces
```mdtest-output
{a:1,b:null(int64)}
{a:2,b:null(int64)}
{a:null(int64),b:3}
```

## Performance

You might think that the overhead involved in managing super-structured types
and the generality of heterogeneous data would confound the performance of
the `super` command, but it turns out that `super` can hold its own when
compared to other analytics systems.

To illustrate comparative performance, we'll present some informal performance
measurements among SuperDB,
[DuckDB](https://duckdb.org/),
[ClickHouse](https://clickhouse.com/), and
[DataFusion](https://datafusion.apache.org/).

We'll use the Parquet format to compare apples to apples
and also report results for the custom columnar database format of DuckDB,
the [new beta JSON type](https://clickhouse.com/blog/a-new-powerful-json-data-type-for-clickhouse) of ClickHouse,
and the [Super Binary](../formats/bsup.md) format used by `super`.

The detailed steps shown [below](#appendix-2-running-the-tests) can be reproduced via
[automated scripts](https://github.com/brimdata/super/blob/main/scripts/super-cmd-perf).
As of this writing in December 2024, [results](#the-test-results) were gathered on an AWS
[`m6idn.2xlarge`](https://aws.amazon.com/ec2/instance-types/m6i/) instance
with the following software versions:

|**Software**|**Version**|
|-|-|
|`super`|Commit `3900a40`|
|`duckdb`|`v1.1.3` 19864453f7|
|`datafusion-cli`|datafusion-cli `43.0.0`|
|`clickhouse`|ClickHouse local version `24.12.1.1614` (official build)|

The complete run logs are [archived here](https://super-cmd-perf.s3.us-east-2.amazonaws.com/2024-12-27_21-58-22.tgz).

### The Test Data

These tests are based on the data and exemplary queries
published by the DuckDB team on their blog
[Shredding Deeply Nested JSON, One Vector at a Time](https://duckdb.org/2023/03/03/json.html).  We'll follow their script starting at the
[GitHub Archive Examples](https://duckdb.org/2023/03/03/json.html#github-archive-examples).

If you want to reproduce these results for yourself,
you can fetch the 2.2GB of gzipped JSON data:
```
wget https://data.gharchive.org/2023-02-08-0.json.gz
wget https://data.gharchive.org/2023-02-08-1.json.gz
...
wget https://data.gharchive.org/2023-02-08-23.json.gz
```
We downloaded these files into a directory called `gharchive_gz`
and created a DuckDB database file called `gha.db` and a table called `gha`
using this command:
```
duckdb gha.db -c "CREATE TABLE gha AS FROM read_json('gharchive_gz/*.json.gz', union_by_name=true)"
```
To create a relational table from the input JSON, we utilized DuckDB's
`union_by_name` parameter to fuse all of the different shapes of JSON encountered
into a single monolithic schema.

We then created a Parquet file called `gha.parquet` with this command:
```
duckdb gha.db -c "COPY (from gha) TO 'gha.parquet'"
```
To create a ClickHouse table using their beta JSON type, after starting
a ClickHouse server we defined the single-column schema before loading the
data using this command:
```
clickhouse-client --query "
  SET enable_json_type = 1;
  CREATE TABLE gha (v JSON) ENGINE MergeTree() ORDER BY tuple();
  INSERT INTO gha SELECT * FROM file('gharchive_gz/*.json.gz', JSONAsObject);"
```
To create a super-structed file for the `super` command, there is no need to
[`fuse`](../language/operators/fuse.md) the data into a single schema (though `super` can still work with the fused
schema in the Parquet file), and we simply ran this command to create a Super Binary
file:
```
super gharchive_gz/*.json.gz > gha.bsup
```
This code path in `super` is not multi-threaded so not particularly performant,
but on our test machine it runs a bit faster than both the `duckdb` method of
creating a schema-fused table or loading the data to the `clickhouse` beta JSON type.

Here are the resulting file sizes:
```
% du -h gha.db gha.parquet gha.bsup gharchive_gz clickhouse/store
9.4G gha.db
4.7G gha.parquet
2.9G gha.bsup
2.3G gharchive_gz
 11G clickhouse/store
```

### The Test Queries

The test queries involve these patterns:
* simple search (single and multicolumn)
* count-where aggregation
* count by field aggregation
* rank over union of disparate field types

We will call these tests [search](#search), [search+](#search-1), [count](#count), [agg](#agg), and [union](#union), respectively

#### Search

For the _search_ test, we'll search for the string pattern
```
    "in case you have any feedback 😊"
```
in the field `payload.pull_request.body`
and we'll just count the number of matches found.
The number of matches is small (2) so the query performance is dominated
by the search.

The SQL for this query is
```sql
SELECT count()
FROM 'gha.parquet' -- or gha
WHERE payload.pull_request.body LIKE '%in case you have any feedback 😊%'
```
To query the data stored with the ClickHouse JSON type, field
references needed to be rewritten relative to the named column `v`.
```sql
SELECT count()
FROM 'gha'
WHERE v.payload.pull_request.body LIKE '%in case you have any feedback 😊%'
```
SuperSQL supports `LIKE` and could run the plain SQL query, but it also has a
similar function called [`grep`](../language/functions/grep.md) that can operate over specified fields or
default to all the string fields in any value. The SuperSQL query that uses
`grep` is
```sql
SELECT count()
FROM 'gha.bsup'
WHERE grep('in case you have any feedback 😊', payload.pull_request.body)
```

#### Search+

For search across multiple columns, SQL doesn't have a `grep` function so
we must enumerate all the fields of such a query.  The SQL for a string search
over our GitHub Archive dataset involves the following fields:
```sql
SELECT count() FROM gha
WHERE id LIKE '%in case you have any feedback 😊%'
  OR type LIKE '%in case you have any feedback 😊%'
  OR actor.login LIKE '%in case you have any feedback 😊%'
  OR actor.display_login LIKE '%in case you have any feedback 😊%'
  ...
  OR payload.member.type LIKE '%in case you have any feedback 😊%'
```
There are 486 such fields.  You can review the entire query in
[`search+.sql`](https://github.com/brimdata/super/blob/main/scripts/super-cmd-perf/queries/search%2B.sql).

To query the data stored with the ClickHouse JSON type, field
references needed to be rewritten relative to the named column `v`.
```sql
SELECT count()
FROM 'gha'
WHERE
   v.id LIKE '%in case you have any feedback 😊%'
   OR v.type LIKE '%in case you have any feedback 😊%'
...
```

In SuperSQL, `grep` allows for a much shorter query.
```sql
SELLECT count()
FROM 'gha.bsup'
WHERE grep('in case you have any feedback 😊')
```

#### Count

In the _count_ test, we filter the input with a `WHERE` clause and count the results.
We chose a random GitHub user name for the filter.
This query has the form:
```sql
SELECT count()
FROM 'gha.parquet' -- or gha or 'gha.bsup'
WHERE actor.login='johnbieren'"
```

To query the data stored with the ClickHouse JSON type, field
references needed to be rewritten relative to the named column `v`.
```sql
SELECT count()
FROM 'gha'
WHERE v.actor.login='johnbieren'
```

#### Agg

In the _agg_ test, we filter the input and count the results grouped by the field `type`
as in the DuckDB blog.
This query has the form:
```sql
SELECT count(),type
FROM 'gha.parquet' -- or 'gha' or 'gha.bsup'
WHERE repo.name='duckdb/duckdb'
GROUP BY type
```

To query the data stored with the ClickHouse JSON type, field
references needed to be rewritten relative to the named column `v`.
```sql
SET allow_suspicious_types_in_group_by = 1;
SELECT count(),v.type
FROM 'gha'
WHERE v.repo.name='duckdb/duckdb'
GROUP BY v.type
```

Also, we had to enable the `allow_suspicious_types_in_group_by` setting as
shown above because an initial attempt to query with default settings
triggered the error:
```
Code: 44. DB::Exception: Received from localhost:9000. DB::Exception: Data
types Variant/Dynamic are not allowed in GROUP BY keys, because it can lead
to unexpected results. Consider using a subcolumn with a specific data type
instead (for example 'column.Int64' or 'json.some.path.:Int64' if its a JSON
path subcolumn) or casting this column to a specific data type. Set setting
allow_suspicious_types_in_group_by = 1 in order to allow it. (ILLEGAL_COLUMN)
```

#### Union

The _union_ test is straight out of the DuckDB blog at the end of
[this section](https://duckdb.org/2023/03/03/json.html#handling-inconsistent-json-schemas).
This query computes the GitHub users that were assigned as a PR reviewer the most often
and returns the top 5 such users.
Because the assignees can appear in either a list of strings
or within a single string field, the relational model requires that two different
subqueries run for the two cases and the result unioned together; then,
this intermediary table can be counted using the unnested
assignee as the group-by key.
This query is:
```sql
WITH assignees AS (
  SELECT payload.pull_request.assignee.login assignee
  FROM 'gha.parquet' -- or 'gha'
  UNION ALL
  SELECT unnest(payload.pull_request.assignees).login assignee
  FROM 'gha.parquet' -- or 'gha'
)
SELECT assignee, count(*) count
FROM assignees
WHERE assignee IS NOT NULL
GROUP BY assignee
ORDER BY count DESC
LIMIT 5
```
For DataFusion, we needed to rewrite this SELECT
```sql
SELECT unnest(payload.pull_request.assignees).login
FROM 'gha.parquet'
```
as
```sql
SELECT object.login as assignee FROM (
    SELECT unnest(payload.pull_request.assignees) object
    FROM 'gha.parquet'
)
```
and for ClickHouse, we had to use `arrayJoin` instead of `unnest`.

Even with this change ClickHouse could only run the query successfully against
the Parquet data, as after rewriting the field references to attempt to
query the data stored with the ClickHouse JSON type it would not run. We
suspect this is likely due to some remaining work in ClickHouse for `arrayJoin`
to work with the new JSON type.
```
$ clickhouse-client --query "
  WITH assignees AS (
    SELECT v.payload.pull_request.assignee.login assignee
    FROM 'gha'
    UNION ALL
    SELECT arrayJoin(v.payload.pull_request.assignees).login assignee
    FROM 'gha'
  )
  SELECT assignee, count(*) count
  FROM assignees
  WHERE assignee IS NOT NULL
  GROUP BY assignee
  ORDER BY count DESC
  LIMIT 5"

Received exception from server (version 24.11.1):
Code: 43. DB::Exception: Received from localhost:9000. DB::Exception: First
argument for function tupleElement must be tuple or array of tuple. Actual
Dynamic: In scope SELECT tupleElement(arrayJoin(v.payload.pull_request.assignees),
'login') AS assignee FROM gha. (ILLEGAL_TYPE_OF_ARGUMENT)
```

SuperSQL's data model does not require these kinds of gymnastics as
everything does not have to be jammed into a table.  Instead, we can use the
`UNNEST` pipe operator combined with the [spread operator](../language/expressions.md#array-expressions) applied to the array of
string fields to easily produce a stream of string values representing the
assignees.  Then we simply aggregate the assignee stream:
```sql
FROM 'gha.bsup'
| UNNEST [...payload.pull_request.assignees, payload.pull_request.assignee]
| WHERE this IS NOT NULL
| AGGREGATE count() BY assignee:=login
| ORDER BY count DESC
| LIMIT 5
```

### The Test Results

The following table summarizes the query performance for each tool as recorded in the
[most recent archived run](https://super-cmd-perf.s3.us-east-2.amazonaws.com/2024-12-27_21-58-22.tgz).
The run time for each query in seconds is shown along with the speed-up factor
in parentheses:

|**Tool**|**Format**|**search**|**search+**|**count**|**agg**|**union**|
|-|-|-|-|-|-|-|
|`super`|`bsup`|6.4<br/>(1.9x)|12.5<br/>(1.6x)|5.8<br/>(0.03x)|5.6<br/>(0.03x)|8.2<br/>(64x)|
|`super`|`parquet`|40.8<br/>(0.3x)|55.1<br/>(0.4x)|0.3<br/>(0.6x)|0.5<br/>(0.3x)|40<br/>(13.2x)|
|`duckdb`|`db`|12.1<br/>(1x)|19.8<br/>(1x)|0.2<br/>(1x)|0.1<br/>(1x)|527<br/>(1x)|
|`duckdb`|`parquet`|13.3<br/>(0.9x)|21.3<br/>(0.9x)|0.4<br/>(0.4x)|0.3<br/>(0.4x)|488<br/>(1.1x)|
|`datafusion`|`parquet`|11.0<br/>(1.1x)|21.2<br/>(0.9x)|0.4<br/>(0.5x)|0.4<br/>(0.4x)|24.2<br/>(22x)|
|`clickhouse`|`parquet`|70<br/>(0.2x)|829<br/>(0.02x)|1.0<br/>(0.2x)|0.9<br/>(0.2x)|71.4<br/>(7x)|
|`clickhouse`|`db`|0.9<br/>(14x)|12.8<br/>(1.6x)|0.1<br/>(2.2x)|0.1<br/>(1.2x)|note|

_Note: we were not able to successfully run the [union query](#union) with
ClickHouse's beta JSON type_

Since DuckDB with its native format could successfully run all queries with
decent performance, we used it as the baseline for all of the speed-up factors.

To summarize,
`super` with Super Binary is substantially faster than multiple relational systems for
the search use cases, and with Parquet performs on par with the others for traditional OLAP queries,
except for the _union_ query, where the super-structured data model trounces the relational
model (by over 60x!) for stitching together disparate data types for analysis in an aggregation.

## Appendix 1: Preparing the Test Data

For our tests, we diverged a bit from the methodology in the DuckDB blog and wanted
to put all the JSON data in a single table.  It wasn't obvious how to go about this
and this section documents the difficulties we encountered trying to do so.

First, we simply tried this:
```
duckdb gha.db -c "CREATE TABLE gha AS FROM 'gharchive_gz/*.json.gz'"
```
which fails with
```
Invalid Input Error: JSON transform error in file "gharchive_gz/2023-02-08-10.json.gz", in line 4903: Object {"url":"https://api.github.com/repos/aws/aws-sam-c... has unknown key "reactions"
Try increasing 'sample_size', reducing 'maximum_depth', specifying 'columns', 'format' or 'records' manually, setting 'ignore_errors' to true, or setting 'union_by_name' to true when reading multiple files with a different structure.
```
Clearly the schema inference algorithm relies upon sampling and the sample doesn't
cover enough data to capture all of its variations.

Okay, maybe there is a reason the blog first explores the structure of
the data to specify `columns` arguments to `read_json` as suggested by the error
message above.  To this end, you can run this query:
```
SELECT json_group_structure(json)
FROM (
  SELECT *
  FROM read_ndjson_objects('gharchive_gz/*.json.gz')
  LIMIT 2048
);
```
Unfortunately, if you use the resulting structure to create the `columns` argument
then `duckdb` fails also because the first 2048 records don't have enough coverage.
So let's try removing the `LIMIT` clause:
```
SELECT json_group_structure(json)
FROM (
  SELECT *
  FROM read_ndjson_objects('gharchive_gz/*.json.gz')
);
```
Hmm, now `duckdb` runs out of memory.

We then thought we'd see if the sampling algorithm of `read_json` is more efficient,
so we tried this command with successively larger sample sizes:
```
duckdb scratch -c "CREATE TABLE gha AS FROM read_json('gharchive_gz/*.json.gz', sample_size=1000000)"
```
Even with a million rows as the sample, `duckdb` fails with
```
Invalid Input Error: JSON transform error in file "gharchive_gz/2023-02-08-14.json.gz", in line 49745: Object {"issues":"write","metadata":"read","pull_requests... has unknown key "repository_hooks"
Try increasing 'sample_size', reducing 'maximum_depth', specifying 'columns', 'format' or 'records' manually, setting 'ignore_errors' to true, or setting 'union_by_name' to true when reading multiple files with a different structure.
```
Ok, there 4,434,953 JSON objects in the input so let's try this:
```
duckdb gha.db -c "CREATE TABLE gha AS FROM read_json('gharchive_gz/*.json.gz', sample_size=4434953)"
```
and again `duckdb` runs out of memory.

So we looked at the other options suggested by the error message and
`union_by_name` appeared promising.  Enabling this option causes DuckDB
to combine all the JSON objects into a single fused schema.
Maybe this would work better?

Sure enough, this works:
```
duckdb gha.db -c "CREATE TABLE gha AS FROM read_json('gharchive_gz/*.json.gz', union_by_name=true)"
```
We now have the DuckDB database file for our GitHub Archive data called `gha.db`
containing a single table called `gha` embedded in that database.
What about the super-structured
format for the `super` command?  There is no need to futz with sample sizes,
schema inference, or union by name. Just run this to create a Super Binary file:
```
super gharchive_gz/*.json.gz > gha.bsup
```

## Appendix 2: Running the Tests

This appendix provides the raw tests and output from the [most recent archived run](https://super-cmd-perf.s3.us-east-2.amazonaws.com/2024-12-27_21-58-22.tgz)
of the tests via [automated scripts](https://github.com/brimdata/super/blob/main/scripts/super-cmd-perf)
on an AWS [`m6idn.2xlarge`](https://aws.amazon.com/ec2/instance-types/m6i/) instance.

### Search Test

```
About to execute
================
clickhouse-client --queries-file /mnt/tmpdir/tmp.NlvDgOOmnG

With query
==========
SELECT count()
FROM 'gha'
WHERE v.payload.pull_request.body LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse-client --queries-file /mnt/tmpdir/tmp.NlvDgOOmnG'
Benchmark 1: clickhouse-client --queries-file /mnt/tmpdir/tmp.NlvDgOOmnG
2
  Time (abs ≡):         0.870 s               [User: 0.045 s, System: 0.023 s]

About to execute
================
clickhouse --queries-file /mnt/tmpdir/tmp.0bwhkb0l9n

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE payload.pull_request.body LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse --queries-file /mnt/tmpdir/tmp.0bwhkb0l9n'
Benchmark 1: clickhouse --queries-file /mnt/tmpdir/tmp.0bwhkb0l9n
2
  Time (abs ≡):        69.650 s               [User: 69.485 s, System: 3.096 s]
 
About to execute
================
datafusion-cli --file /mnt/tmpdir/tmp.S0ITz1nHQG

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE payload.pull_request.body LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'datafusion-cli --file /mnt/tmpdir/tmp.S0ITz1nHQG'
Benchmark 1: datafusion-cli --file /mnt/tmpdir/tmp.S0ITz1nHQG
DataFusion CLI v43.0.0
+---------+
| count() |
+---------+
| 2       |
+---------+
1 row(s) fetched. 
Elapsed 10.811 seconds.

  Time (abs ≡):        11.041 s               [User: 65.647 s, System: 11.209 s]
 
About to execute
================
duckdb /mnt/gha.db < /mnt/tmpdir/tmp.wsNTlXhTTF

With query
==========
SELECT count()
FROM 'gha'
WHERE payload.pull_request.body LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb /mnt/gha.db < /mnt/tmpdir/tmp.wsNTlXhTTF'
Benchmark 1: duckdb /mnt/gha.db < /mnt/tmpdir/tmp.wsNTlXhTTF
┌──────────────┐
│ count_star() │
│    int64     │
├──────────────┤
│            2 │
└──────────────┘
  Time (abs ≡):        12.051 s               [User: 78.680 s, System: 8.891 s]
 
About to execute
================
duckdb < /mnt/tmpdir/tmp.hPiKS1Qi1A

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE payload.pull_request.body LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb < /mnt/tmpdir/tmp.hPiKS1Qi1A'
Benchmark 1: duckdb < /mnt/tmpdir/tmp.hPiKS1Qi1A
┌──────────────┐
│ count_star() │
│    int64     │
├──────────────┤
│            2 │
└──────────────┘
  Time (abs ≡):        13.267 s               [User: 90.148 s, System: 6.506 s]
 
About to execute
================
super -z -I /mnt/tmpdir/tmp.pDeSZCTa2V

With query
==========
SELECT count()
FROM '/mnt/gha.bsup'
WHERE grep('in case you have any feedback 😊', payload.pull_request.body)

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'super -z -I /mnt/tmpdir/tmp.pDeSZCTa2V'
Benchmark 1: super -z -I /mnt/tmpdir/tmp.pDeSZCTa2V
{count:2(uint64)}
  Time (abs ≡):         6.371 s               [User: 23.178 s, System: 1.700 s]
 
About to execute
================
SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.AYZIh6yi2s

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE grep('in case you have any feedback 😊', payload.pull_request.body)

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.AYZIh6yi2s'
Benchmark 1: SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.AYZIh6yi2s
{count:2(uint64)}
  Time (abs ≡):        40.838 s               [User: 292.674 s, System: 18.797 s]
```
### Search+ Test

```
About to execute
================
clickhouse-client --queries-file /mnt/tmpdir/tmp.PFNN1fKojv

With query
==========
SELECT count()
FROM 'gha'
WHERE
   v.id LIKE '%in case you have any feedback 😊%'
   OR v.type LIKE '%in case you have any feedback 😊%'
   ...
   OR v.payload.member.type LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse-client --queries-file /mnt/tmpdir/tmp.PFNN1fKojv'
Benchmark 1: clickhouse-client --queries-file /mnt/tmpdir/tmp.PFNN1fKojv
3
  Time (abs ≡):        12.773 s               [User: 0.061 s, System: 0.025 s]

About to execute
================
clickhouse --queries-file /mnt/tmpdir/tmp.PTRkZ4ZIXX

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE
   id LIKE '%in case you have any feedback 😊%'
   OR type LIKE '%in case you have any feedback 😊%'
   ...
   OR payload.member.type LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse --queries-file /mnt/tmpdir/tmp.PTRkZ4ZIXX'
Benchmark 1: clickhouse --queries-file /mnt/tmpdir/tmp.PTRkZ4ZIXX
3
  Time (abs ≡):        828.691 s               [User: 908.452 s, System: 17.692 s]
 
About to execute
================
datafusion-cli --file /mnt/tmpdir/tmp.SCtJ9sNeBA

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE
   id LIKE '%in case you have any feedback 😊%'
   OR type LIKE '%in case you have any feedback 😊%'
   ...
   OR payload.member.type LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'datafusion-cli --file /mnt/tmpdir/tmp.SCtJ9sNeBA'
Benchmark 1: datafusion-cli --file /mnt/tmpdir/tmp.SCtJ9sNeBA
DataFusion CLI v43.0.0
+---------+
| count() |
+---------+
| 3       |
+---------+
1 row(s) fetched. 
Elapsed 20.990 seconds.

  Time (abs ≡):        21.228 s               [User: 127.034 s, System: 19.513 s]
 
About to execute
================
duckdb /mnt/gha.db < /mnt/tmpdir/tmp.SXkIoC2XJo

With query
==========
SELECT count()
FROM 'gha'
WHERE
   id LIKE '%in case you have any feedback 😊%'
   OR type LIKE '%in case you have any feedback 😊%'
   ...
   OR payload.member.type LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb /mnt/gha.db < /mnt/tmpdir/tmp.SXkIoC2XJo'
Benchmark 1: duckdb /mnt/gha.db < /mnt/tmpdir/tmp.SXkIoC2XJo
┌──────────────┐
│ count_star() │
│    int64     │
├──────────────┤
│            3 │
└──────────────┘
  Time (abs ≡):        19.814 s               [User: 140.302 s, System: 9.875 s]
 
About to execute
================
duckdb < /mnt/tmpdir/tmp.k6yVjzT4cu

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE
   id LIKE '%in case you have any feedback 😊%'
   OR type LIKE '%in case you have any feedback 😊%'
   ...
   OR payload.member.type LIKE '%in case you have any feedback 😊%'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb < /mnt/tmpdir/tmp.k6yVjzT4cu'
Benchmark 1: duckdb < /mnt/tmpdir/tmp.k6yVjzT4cu
┌──────────────┐
│ count_star() │
│    int64     │
├──────────────┤
│            3 │
└──────────────┘
  Time (abs ≡):        21.286 s               [User: 145.120 s, System: 8.677 s]
 
About to execute
================
super -z -I /mnt/tmpdir/tmp.jJSibCjp8r

With query
==========
SELECT count()
FROM '/mnt/gha.bsup'
WHERE grep('in case you have any feedback 😊')

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'super -z -I /mnt/tmpdir/tmp.jJSibCjp8r'
Benchmark 1: super -z -I /mnt/tmpdir/tmp.jJSibCjp8r
{count:3(uint64)}
  Time (abs ≡):        12.492 s               [User: 88.901 s, System: 1.672 s]
 
About to execute
================
SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.evXq1mxkI0

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE grep('in case you have any feedback 😊')

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.evXq1mxkI0'
Benchmark 1: SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.evXq1mxkI0
{count:3(uint64)}
  Time (abs ≡):        55.081 s               [User: 408.337 s, System: 18.597 s]
```

### Count Test

```
About to execute
================
clickhouse-client --queries-file /mnt/tmpdir/tmp.Wqytp5T3II

With query
==========
SELECT count()
FROM 'gha'
WHERE v.actor.login='johnbieren'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse-client --queries-file /mnt/tmpdir/tmp.Wqytp5T3II'
Benchmark 1: clickhouse-client --queries-file /mnt/tmpdir/tmp.Wqytp5T3II
879
  Time (abs ≡):         0.081 s               [User: 0.021 s, System: 0.023 s]
 
About to execute
================
clickhouse --queries-file /mnt/tmpdir/tmp.O95s9fJprP

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE actor.login='johnbieren'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse --queries-file /mnt/tmpdir/tmp.O95s9fJprP'
Benchmark 1: clickhouse --queries-file /mnt/tmpdir/tmp.O95s9fJprP
879
  Time (abs ≡):         0.972 s               [User: 0.836 s, System: 0.156 s]
 
About to execute
================
datafusion-cli --file /mnt/tmpdir/tmp.CHTPCdHbaG

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE actor.login='johnbieren'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'datafusion-cli --file /mnt/tmpdir/tmp.CHTPCdHbaG'
Benchmark 1: datafusion-cli --file /mnt/tmpdir/tmp.CHTPCdHbaG
DataFusion CLI v43.0.0
+---------+
| count() |
+---------+
| 879     |
+---------+
1 row(s) fetched. 
Elapsed 0.340 seconds.

  Time (abs ≡):         0.384 s               [User: 1.600 s, System: 0.409 s]
 
About to execute
================
duckdb /mnt/gha.db < /mnt/tmpdir/tmp.VQ2IgDaeUO

With query
==========
SELECT count()
FROM 'gha'
WHERE actor.login='johnbieren'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb /mnt/gha.db < /mnt/tmpdir/tmp.VQ2IgDaeUO'
Benchmark 1: duckdb /mnt/gha.db < /mnt/tmpdir/tmp.VQ2IgDaeUO
┌──────────────┐
│ count_star() │
│    int64     │
├──────────────┤
│          879 │
└──────────────┘
  Time (abs ≡):         0.178 s               [User: 1.070 s, System: 0.131 s]
 
About to execute
================
duckdb < /mnt/tmpdir/tmp.rjFqrZFUtF

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE actor.login='johnbieren'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb < /mnt/tmpdir/tmp.rjFqrZFUtF'
Benchmark 1: duckdb < /mnt/tmpdir/tmp.rjFqrZFUtF
┌──────────────┐
│ count_star() │
│    int64     │
├──────────────┤
│          879 │
└──────────────┘
  Time (abs ≡):         0.426 s               [User: 2.252 s, System: 0.194 s]
 
About to execute
================
super -z -I /mnt/tmpdir/tmp.AbeKpBbYW8

With query
==========
SELECT count()
FROM '/mnt/gha.bsup'
WHERE actor.login='johnbieren'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'super -z -I /mnt/tmpdir/tmp.AbeKpBbYW8'
Benchmark 1: super -z -I /mnt/tmpdir/tmp.AbeKpBbYW8
{count:879(uint64)}
  Time (abs ≡):         5.786 s               [User: 17.405 s, System: 1.637 s]
 
About to execute
================
SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.5xTnB02WgG

With query
==========
SELECT count()
FROM '/mnt/gha.parquet'
WHERE actor.login='johnbieren'

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.5xTnB02WgG'
Benchmark 1: SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.5xTnB02WgG
{count:879(uint64)}
  Time (abs ≡):         0.303 s               [User: 0.792 s, System: 0.240 s]
```

### Agg Test

```
About to execute
================
clickhouse --queries-file /mnt/tmpdir/tmp.k2UT3NLBd6

With query
==========
SELECT count(),type
FROM '/mnt/gha.parquet'
WHERE repo.name='duckdb/duckdb'
GROUP BY type

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse --queries-file /mnt/tmpdir/tmp.k2UT3NLBd6'
Benchmark 1: clickhouse --queries-file /mnt/tmpdir/tmp.k2UT3NLBd6
30	IssueCommentEvent
14	PullRequestReviewEvent
29	WatchEvent
15	PushEvent
7	PullRequestReviewCommentEvent
9	IssuesEvent
3	ForkEvent
35	PullRequestEvent
  Time (abs ≡):         0.860 s               [User: 0.757 s, System: 0.172 s]
 
About to execute
================
clickhouse-client --queries-file /mnt/tmpdir/tmp.MqFw3Iihza

With query
==========
SET allow_suspicious_types_in_group_by = 1;
SELECT count(),v.type
FROM 'gha'
WHERE v.repo.name='duckdb/duckdb'
GROUP BY v.type

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse-client --queries-file /mnt/tmpdir/tmp.MqFw3Iihza'
Benchmark 1: clickhouse-client --queries-file /mnt/tmpdir/tmp.MqFw3Iihza
14	PullRequestReviewEvent
15	PushEvent
9	IssuesEvent
3	ForkEvent
7	PullRequestReviewCommentEvent
29	WatchEvent
30	IssueCommentEvent
35	PullRequestEvent
  Time (abs ≡):         0.122 s               [User: 0.032 s, System: 0.019 s]
 
About to execute
================
datafusion-cli --file /mnt/tmpdir/tmp.Rf1BJWypeQ

With query
==========
SELECT count(),type
FROM '/mnt/gha.parquet'
WHERE repo.name='duckdb/duckdb'
GROUP BY type

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'datafusion-cli --file /mnt/tmpdir/tmp.Rf1BJWypeQ'
Benchmark 1: datafusion-cli --file /mnt/tmpdir/tmp.Rf1BJWypeQ
DataFusion CLI v43.0.0
+---------+-------------------------------+
| count() | type                          |
+---------+-------------------------------+
| 29      | WatchEvent                    |
| 3       | ForkEvent                     |
| 35      | PullRequestEvent              |
| 14      | PullRequestReviewEvent        |
| 7       | PullRequestReviewCommentEvent |
| 30      | IssueCommentEvent             |
| 9       | IssuesEvent                   |
| 15      | PushEvent                     |
+---------+-------------------------------+
8 row(s) fetched. 
Elapsed 0.320 seconds.

  Time (abs ≡):         0.365 s               [User: 1.399 s, System: 0.399 s]
 
About to execute
================
duckdb /mnt/gha.db < /mnt/tmpdir/tmp.pEWjK5q2sA

With query
==========
SELECT count(),type
FROM 'gha'
WHERE repo.name='duckdb/duckdb'
GROUP BY type

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb /mnt/gha.db < /mnt/tmpdir/tmp.pEWjK5q2sA'
Benchmark 1: duckdb /mnt/gha.db < /mnt/tmpdir/tmp.pEWjK5q2sA
┌──────────────┬───────────────────────────────┐
│ count_star() │             type              │
│    int64     │            varchar            │
├──────────────┼───────────────────────────────┤
│           14 │ PullRequestReviewEvent        │
│           29 │ WatchEvent                    │
│           30 │ IssueCommentEvent             │
│           15 │ PushEvent                     │
│            9 │ IssuesEvent                   │
│            7 │ PullRequestReviewCommentEvent │
│            3 │ ForkEvent                     │
│           35 │ PullRequestEvent              │
└──────────────┴───────────────────────────────┘
  Time (abs ≡):         0.141 s               [User: 0.756 s, System: 0.147 s]
 
About to execute
================
duckdb < /mnt/tmpdir/tmp.cC0xpHh2ee

With query
==========
SELECT count(),type
FROM '/mnt/gha.parquet'
WHERE repo.name='duckdb/duckdb'
GROUP BY type

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb < /mnt/tmpdir/tmp.cC0xpHh2ee'
Benchmark 1: duckdb < /mnt/tmpdir/tmp.cC0xpHh2ee
┌──────────────┬───────────────────────────────┐
│ count_star() │             type              │
│    int64     │            varchar            │
├──────────────┼───────────────────────────────┤
│            3 │ ForkEvent                     │
│           14 │ PullRequestReviewEvent        │
│           15 │ PushEvent                     │
│            9 │ IssuesEvent                   │
│            7 │ PullRequestReviewCommentEvent │
│           29 │ WatchEvent                    │
│           30 │ IssueCommentEvent             │
│           35 │ PullRequestEvent              │
└──────────────┴───────────────────────────────┘
  Time (abs ≡):         0.320 s               [User: 1.529 s, System: 0.175 s]
 
About to execute
================
super -z -I /mnt/tmpdir/tmp.QMhaBvUi2y

With query
==========
SELECT count(),type
FROM '/mnt/gha.bsup'
WHERE repo.name='duckdb/duckdb'
GROUP BY type

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'super -z -I /mnt/tmpdir/tmp.QMhaBvUi2y'
Benchmark 1: super -z -I /mnt/tmpdir/tmp.QMhaBvUi2y
{type:"PullRequestReviewCommentEvent",count:7(uint64)}
{type:"PullRequestReviewEvent",count:14(uint64)}
{type:"IssueCommentEvent",count:30(uint64)}
{type:"WatchEvent",count:29(uint64)}
{type:"PullRequestEvent",count:35(uint64)}
{type:"PushEvent",count:15(uint64)}
{type:"IssuesEvent",count:9(uint64)}
{type:"ForkEvent",count:3(uint64)}
  Time (abs ≡):         5.626 s               [User: 15.509 s, System: 1.552 s]
 
About to execute
================
SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.yfAdMeskPR

With query
==========
SELECT count(),type
FROM '/mnt/gha.parquet'
WHERE repo.name='duckdb/duckdb'
GROUP BY type

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.yfAdMeskPR'
Benchmark 1: SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.yfAdMeskPR
{type:"PushEvent",count:15(uint64)}
{type:"IssuesEvent",count:9(uint64)}
{type:"WatchEvent",count:29(uint64)}
{type:"PullRequestEvent",count:35(uint64)}
{type:"ForkEvent",count:3(uint64)}
{type:"PullRequestReviewCommentEvent",count:7(uint64)}
{type:"PullRequestReviewEvent",count:14(uint64)}
{type:"IssueCommentEvent",count:30(uint64)}
  Time (abs ≡):         0.491 s               [User: 2.049 s, System: 0.357 s]
```

### Union Test

```
About to execute
================
clickhouse --queries-file /mnt/tmpdir/tmp.6r4kTKMn1T

With query
==========
WITH assignees AS (
  SELECT payload.pull_request.assignee.login assignee
  FROM '/mnt/gha.parquet'
  UNION ALL
  SELECT arrayJoin(payload.pull_request.assignees).login assignee
  FROM '/mnt/gha.parquet'
)
SELECT assignee, count(*) count
FROM assignees
WHERE assignee IS NOT NULL
GROUP BY assignee
ORDER BY count DESC
LIMIT 5

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'clickhouse --queries-file /mnt/tmpdir/tmp.6r4kTKMn1T'
Benchmark 1: clickhouse --queries-file /mnt/tmpdir/tmp.6r4kTKMn1T
poad	1966
vinayakkulkarni	508
tmtmtmtm	356
AMatutat	260
danwinship	208
  Time (abs ≡):        71.372 s               [User: 142.043 s, System: 6.278 s]
 
About to execute
================
datafusion-cli --file /mnt/tmpdir/tmp.GgJzlAtf6a

With query
==========
WITH assignees AS (
  SELECT payload.pull_request.assignee.login assignee
  FROM '/mnt/gha.parquet'
  UNION ALL
  SELECT object.login as assignee FROM (
    SELECT unnest(payload.pull_request.assignees) object
    FROM '/mnt/gha.parquet'
  )
)
SELECT assignee, count() count
FROM assignees
WHERE assignee IS NOT NULL
GROUP BY assignee
ORDER BY count DESC
LIMIT 5

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'datafusion-cli --file /mnt/tmpdir/tmp.GgJzlAtf6a'
Benchmark 1: datafusion-cli --file /mnt/tmpdir/tmp.GgJzlAtf6a
DataFusion CLI v43.0.0
+-----------------+-------+
| assignee        | count |
+-----------------+-------+
| poad            | 1966  |
| vinayakkulkarni | 508   |
| tmtmtmtm        | 356   |
| AMatutat        | 260   |
| danwinship      | 208   |
+-----------------+-------+
5 row(s) fetched. 
Elapsed 23.907 seconds.

  Time (abs ≡):        24.215 s               [User: 163.583 s, System: 24.973 s]
 
About to execute
================
duckdb /mnt/gha.db < /mnt/tmpdir/tmp.Q49a92Gvr5

With query
==========
WITH assignees AS (
  SELECT payload.pull_request.assignee.login assignee
  FROM 'gha'
  UNION ALL
  SELECT unnest(payload.pull_request.assignees).login assignee
  FROM 'gha'
)
SELECT assignee, count(*) count
FROM assignees
WHERE assignee IS NOT NULL
GROUP BY assignee
ORDER BY count DESC
LIMIT 5

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb /mnt/gha.db < /mnt/tmpdir/tmp.Q49a92Gvr5'
Benchmark 1: duckdb /mnt/gha.db < /mnt/tmpdir/tmp.Q49a92Gvr5
┌─────────────────┬───────┐
│    assignee     │ count │
│     varchar     │ int64 │
├─────────────────┼───────┤
│ poad            │  1966 │
│ vinayakkulkarni │   508 │
│ tmtmtmtm        │   356 │
│ AMatutat        │   260 │
│ danwinship      │   208 │
└─────────────────┴───────┘
  Time (abs ≡):        527.130 s               [User: 4056.419 s, System: 15.145 s]
 
About to execute
================
duckdb < /mnt/tmpdir/tmp.VQYM2LCNeB

With query
==========
WITH assignees AS (
  SELECT payload.pull_request.assignee.login assignee
  FROM '/mnt/gha.parquet'
  UNION ALL
  SELECT unnest(payload.pull_request.assignees).login assignee
  FROM '/mnt/gha.parquet'
)
SELECT assignee, count(*) count
FROM assignees
WHERE assignee IS NOT NULL
GROUP BY assignee
ORDER BY count DESC
LIMIT 5

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'duckdb < /mnt/tmpdir/tmp.VQYM2LCNeB'
Benchmark 1: duckdb < /mnt/tmpdir/tmp.VQYM2LCNeB
┌─────────────────┬───────┐
│    assignee     │ count │
│     varchar     │ int64 │
├─────────────────┼───────┤
│ poad            │  1966 │
│ vinayakkulkarni │   508 │
│ tmtmtmtm        │   356 │
│ AMatutat        │   260 │
│ danwinship      │   208 │
└─────────────────┴───────┘
  Time (abs ≡):        488.127 s               [User: 3660.271 s, System: 10.031 s]
 
About to execute
================
super -z -I /mnt/tmpdir/tmp.JzRx6IABuv

With query
==========
FROM '/mnt/gha.bsup'
| UNNEST [...payload.pull_request.assignees, payload.pull_request.assignee]
| WHERE this IS NOT NULL
| AGGREGATE count() BY assignee:=login
| ORDER BY count DESC
| LIMIT 5

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'super -z -I /mnt/tmpdir/tmp.JzRx6IABuv'
Benchmark 1: super -z -I /mnt/tmpdir/tmp.JzRx6IABuv
{assignee:"poad",count:1966(uint64)}
{assignee:"vinayakkulkarni",count:508(uint64)}
{assignee:"tmtmtmtm",count:356(uint64)}
{assignee:"AMatutat",count:260(uint64)}
{assignee:"danwinship",count:208(uint64)}
  Time (abs ≡):         8.245 s               [User: 17.489 s, System: 1.938 s]
 
About to execute
================
SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.djiUKncZ0T

With query
==========
FROM '/mnt/gha.parquet'
| UNNEST [...payload.pull_request.assignees, payload.pull_request.assignee]
| WHERE this IS NOT NULL
| AGGREGATE count() BY assignee:=login
| ORDER BY count DESC
| LIMIT 5

+ hyperfine --show-output --warmup 1 --runs 1 --time-unit second 'SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.djiUKncZ0T'
Benchmark 1: SUPER_VAM=1 super -z -I /mnt/tmpdir/tmp.djiUKncZ0T
{assignee:"poad",count:1966(uint64)}
{assignee:"vinayakkulkarni",count:508(uint64)}
{assignee:"tmtmtmtm",count:356(uint64)}
{assignee:"AMatutat",count:260(uint64)}
{assignee:"danwinship",count:208(uint64)}
  Time (abs ≡):        40.014 s               [User: 291.269 s, System: 17.516 s]
```
