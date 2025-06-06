# Command Usage

TODO: REWORK THIS FOCUSING ON A SIMPLER EXPLANATION OF THE COMMAND-LINE TOOLING 
AND BREAK OUT TUTORIALISH TEXT AND PERF STUFF INTO SEPARATE SECTIONS


## Synopsis

`super` is a command-line tool that uses [SuperSQL](../language/_index.md)
to query a variety of data formats in files, over HTTP, or in [S3](../integrations/amazon-s3.md)
storage. Best performance is achieved when operating on data in binary formats such as
[Super Binary (BSUP)](../formats/bsup.md), [Super Columnar (CSUP)](../formats/csup.md),
[Parquet](https://github.com/apache/parquet-format), or
[Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format).

> The SuperDB code and docs are still under construction. Once you've [installed](../getting_started/install.md) `super` we
> recommend focusing first on the functionality shown in this page. Feel free to
> explore other docs and try things out, but please don't be shocked if you hit
> speedbumps in the near term, particularly in areas like performance and full
> SQL coverage. We're working on it! 😉
> 
> Once you've tried it out, we'd love to
> hear your feedback via our [community Slack](https://www.brimdata.io/join-slack/).

## Usage

```
super [ options ] [ -c query ] input [ input ... ]
```

`super` is a command-line tool for processing data in diverse input
formats, providing data wrangling, search, analytics, and extensive transformations
using the [SuperSQL](../language/_index.md) dialect of SQL. Any SQL query expression
may be extended with [pipe syntax](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/)
to filter, transform, and/or analyze input data.
Super's SQL pipes dialect is extensive, so much so that it can resemble
a log-search experience despite its SQL foundation.

The `super` command works with data from ephemeral sources like files and URLs.
If you want to persist your data into a data lake for persistent storage,
check out the [`super db`](super-db.md) set of commands.

By invoking the `-c` option, a query expressed in the [SuperSQL language](../language/_index.md)
may be specified and applied to the input stream.

The [super data model](../formats/data-model.md) is based on [super-structured data](../formats/_index.md#2-a-super-structured-pattern), meaning that all data
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
super -c "FROM example.json | SELECT a,b,c"
```
is equivalent to
```
super -c "SELECT a,b,c" example.json
```
and both are equivalent to the classic SQL
```
super -c "SELECT a,b,c FROM example.json"
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
super -s -c '1+1'
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

`super` supports a number of [input](#input-formats) and [output](#output-formats) formats, but the
[SUP](../formats/sup.md),
[BSUP](../formats/bsup.md), and
[CSUP](../formats/csup.md) formats tend to be the most versatile and
easy to work with.

`super` typically operates on binary-encoded data and when you want to inspect
human-readable bits of output, you merely format it as SUP, which is the
default format when output is directed to the terminal.  BSUP is the default
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
| `bsup`    |  yes | [BSUP](../formats/bsup.md) |
| `csup`    |  yes | [CSUP](../formats/csup.md) |
| `csv`     |  yes | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `json`    |  yes | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
| `line`    |  no  | One string value per input line |
| `parquet` |  yes | [Apache Parquet](https://github.com/apache/parquet-format) |
| `sup`     |  yes | [SUP](../formats/sup.md) |
| `tsv`     |  yes | [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values) |
| `zeek`    |  yes | [Zeek Logs](https://docs.zeek.org/en/master/logs/index.html) |
| `zjson`   |  yes | [Super over JSON (JSUP)](../formats/zjson.md) |

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
super -s sample.csv sample.json
```
would produce this output in the default SUP format
```mdtest-output
{a:1.,b:"foo"}
{a:2.,b:"bar"}
{a:3,b:"baz"}
```

#### JSON Auto-detection: Super vs. Plain

Since [SUP](../formats/sup.md) is a superset of plain JSON, `super` must be careful how it distinguishes the two cases when performing auto-inference.
While you can always clarify your intent
via `-i sup` or `-i json`, `super` attempts to "just do the right thing"
when you run it with SUP vs. plain JSON.

While `super` can parse any JSON using its built-in SUP parser this is typically
not desirable because (1) the SUP parser is not particularly performant and
(2) all JSON numbers are floating point but the SUP parser will parse as
JSON any number that appears without a decimal point as an integer type.

> The reason `super` is not particularly performant for SUP is that the [BSUP](../formats/bsup.md) or
> [CSUP](../formats/csup.md) formats are semantically equivalent to SUP but much more efficient and
> the design intent is that these efficient binary formats should be used in
> use cases where performance matters.  SUP is typically used only when
> data needs to be human-readable in interactive settings or in automated tests.

To this end, `super` uses a heuristic to select between SUP and plain JSON when the
`-i` option is not specified. Specifically, plain JSON is selected when the first values
of the input are parsable as valid JSON and includes a JSON object either
as an outer object or as a value nested somewhere within a JSON array.

This heuristic almost always works in practice because SUP records
typically omit quotes around field names.

### Output Formats

`super` currently supports the following output formats:

|  Option   | Specification                            |
|-----------|------------------------------------------|
| `arrows`  | [Arrow IPC Stream Format](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format) |
| `bsup`    | [BSUP](../formats/bsup.md) |
| `csup`    | [CSUP](../formats/csup.md) |
| `csv`     | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `json`    | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
| `lake`    | [SuperDB Data Lake Metadata Output](#superdb-data-lake-metadata-output) |
| `line`    | (described [below](#simplified-text-outputs)) |
| `parquet` | [Apache Parquet](https://github.com/apache/parquet-format) |
| `sup`     | [SUP](../formats/sup.md) |
| `table`   | (described [below](#simplified-text-outputs)) |
| `text`    | (described [below](#simplified-text-outputs)) |
| `tsv`     | [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values) |
| `zeek`    | [Zeek Logs](https://docs.zeek.org/en/master/logs/index.html) |
| `zjson`   | [SUP over JSON (JSUP)](../formats/zjson.md) |

The output format defaults to either SUP or BSUP and may be specified
with the `-f` option.

Since SUP is a common format choice, the `-s` flag is a shortcut for
`-f sup`.  Also, `-S` is a shortcut for `-f sup` with `-pretty 4` as
[described below](#pretty-printing).

And since plain JSON is another common format choice, the `-j` flag is a shortcut for
`-f json` and `-J` is a shortcut for pretty printing JSON.

#### Output Format Selection

When the format is not specified with `-f`, it defaults to SUP if the output
is a terminal and to BSUP otherwise.

While this can cause an occasional surprise (e.g., forgetting `-f` or `-s`
in a scripted test that works fine on the command line but fails in CI),
we felt that the design of having a uniform default had worse consequences:
* If the default format were SUP, it would be very easy to create pipelines
and deploy to production systems that were accidentally using SUP instead of
the much more efficient BSUP format because the `-f bsup` had been mistakenly
omitted from some command.  The beauty of SuperDB is that all of this "just works"
but it would otherwise perform poorly.
* If the default format were BSUP, then users would be endlessly annoyed by
binary output to their terminal when forgetting to type `-f sup`.

In practice, we have found that the output defaults
"just do the right thing" almost all of the time.

#### Pretty Printing

SUP and plain JSON text may be "pretty printed" with the `-pretty` option, which takes
the number of spaces to use for indentation.  As this is a common option,
the `-S` option is a shortcut for `-f sup -pretty 4` and `-J` is a shortcut
for `-f json -pretty 4`.

For example,
```mdtest-command
echo '{a:{b:1,c:[1,2]},d:"foo"}' | super -S -
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
echo '{a:{b:1,c:[1,2]},d:"foo"}' | super -f sup -pretty 2 -
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

TODO: MOVE THIS STUFF INTO TOP INTRO... SELF-DESCRIBING FORMATS

#### Pipeline-friendly BSUP

Though it's a compressed format, BSUP data is self-describing and stream-oriented
and thus is pipeline friendly.

Since data is self-describing you can simply take BSUP output
of one command and pipe it to the input of another.  It doesn't matter if the value
sequence is scalars, complex types, or records.  There is no need to declare
or register schemas or "protos" with the downstream entities.

In particular, BSUP data can simply be concatenated together, e.g.,
```mdtest-command
super -f bsup -c 'select value 1, [1,2,3]' > a.bsup
super -f bsup -c "select value {s:'hello'}, {s:'world'}" > b.bsup
cat a.bsup b.bsup | super -s -
```
produces
```mdtest-output
1
[1,2,3]
{s:"hello"}
{s:"world"}
```
And while this SUP output is human readable, the BSUP files are binary, e.g.,
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
super -s out.parquet
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
super -s out-*.parquet
```
produces the original data
```mdtest-output
{x:1}
{s:"hello"}
```

While the `-split` option is most useful for schema-rigid formats, it can
be used with any output format.

#### Simplified Text Outputs

The `line`, `text`, and `table` formats simplify data to fit within the
limitations of text-based output. They may be a good fit for use with other text-based shell
tools, but due to their limitations should be used with care.

In `line` output, each string value is printed on its own line, with minimal
formatting applied if any of the following escape sequences are present:

| Escape Sequence | Rendered As                             |
|-----------------|-----------------------------------------|
| `\n`            | Newline                                 |
| `\t`            | Horizontal tab                          |
| `\\`            | Backslash                               |
| `\"`            | Double quote                            |
| `\r`            | Carriage return                         |
| `\b`            | Backspace                               |
| `\f`            | Form feed                               |
| `\u`            | Unicode escape (e.g., `\u0041` for `A`) |

Non-string values are formatted as [SUP](../formats/sup.md).

For example:

```mdtest-command
echo '"hi" "hello\nworld" { time_elapsed: 86400s }' | super -f line -
```
produces
```mdtest-output
hi
hello
world
{time_elapsed:1d}
```

In `text` output, minimal formatting is applied, e.g., strings are shown
without quotes and brackets are dropped from [arrays](../formats/data-model.md#22-array)
and [sets](../formats/data-model.md#23-set). [Records](../formats/data-model.md#21-record)
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
`super db query -S "from :pools"` and saved the output in this file `pools.sup`.

```mdtest-input pools.sup
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
super -f lake pools.sup
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
echo '1 2 0 3' | super -s -c '10.0/this' -
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
echo '1 2 0 3' | super -c '10.0/this' - | super -s -c 'is_error(this)' -
```
produces just
```mdtest-output
error("divide by zero")
```

## Examples

As you may have noticed, many examples shown above were illustrated using this
pattern:
```
echo <values> | super -c <query> -
```

While this is suitable for showing command line operations, in documentation
focused on showing the [SuperSQL language](../language/_index.md) (such as the
[operator](../language/operators/_index.md) and [function](../language/functions/_index.md)
references), interactive examples are often used that are pre-populated with
the query, input, and a "live" result that's generated using a
[browser-based packaging of the SuperDB runtime](https://github.com/brimdata/superdb-wasm).
This allows you to modify the query and/or inputs and immediately see how it
changes the result, all without having to drop to the shell. If you make
changes to an example and want to return it to its original state, just
refresh the page in your browser.

For example, here's an interactive rendering of the example from the
[Error Handling](#error-handling) section above. Try adding an additional
input value and notice the immediate change in the result.

```mdtest-spq
# spq
10.0/this
# input
1
2
0
3
# expected output
10.
5.
error("divide by zero")
3.3333333333333335
```

To run an example in a shell rather than the browser, clicking the "CLI" tab
will format it as a `super` command line. Hovering your mouse pointer over the
panel will reveal a "COPY" button that can be clicked to bring the contents
into your paste buffer so you can easily transfer it into a Bash (or similar)
shell for execution.

The language documentation and [tutorials directory](../tutorials/_index.md)
have many examples, but here are a few more simple `super` use cases.

_Hello, world_
```mdtest-command
super -s -c "SELECT VALUE 'hello, world'"
```
produces this SUP output
```mdtest-output
"hello, world"
```

_Some values of available [data types](../language/data-types.md)_
```mdtest-spq
# spq
SELECT VALUE in
# input
{in:1}
{in:1.5}
{in:[1,"foo"]}
{in:|["apple","banana"]|}
# expected output
1
1.5
[1,"foo"]
|["apple","banana"]|
```

_The types of various data_
```mdtest-spq
# spq
SELECT VALUE typeof(in)
# input
{in:1}
{in:1.5}
{in:[1,"foo"]}
{in:|["apple","banana"]|}
# expected output
<int64>
<float64>
<[(int64,string)]>
<|[string]|>
```

_A simple [aggregation](../language/aggregates/_index.md)_
```mdtest-spq
# spq
sum(val) by key | sort key
# input
{key:"foo",val:1}
{key:"bar",val:2}
{key:"foo",val:3}
# expected output
{key:"bar",sum:2}
{key:"foo",sum:4}
```

_Read CSV input and [cast](../language/functions/cast.md) a to an integer from default float_
```mdtest-spq
# spq
a:=int64(a)
# input
a,b
1,foo
2,bar
# expected output
{a:1,b:"foo"}
{a:2,b:"bar"}
```

_Read JSON input and cast to an integer from default float_
```mdtest-spq
# spq
a:=int64(a)
# input
{"a":1,"b":"foo"}
{"a":2,"b":"bar"}
# expected output
{a:1,b:"foo"}
{a:2,b:"bar"}
```

_Make a schema-rigid Parquet file using fuse, then output the Parquet file as SUP_
```mdtest-command
echo '{a:1}{a:2}{b:3}' | super -f parquet -o tmp.parquet -c fuse -
super -s tmp.parquet
```
produces
```mdtest-output
{a:1,b:null(int64)}
{a:2,b:null(int64)}
{a:null(int64),b:3}
```


TODO: MERGE THE SUPER DB SECTION WITH ABOVE.  FOCUS ON HOW TO USE THE COMMANDS 
NOT TEACHING THE USER ABOUT SUPERSQL.


## Synopsis

`super db` is a sub-command of [`super`](./super.md) to manage and query SuperDB data lakes.
You can import data from a variety of formats and it will automatically
be committed in [super-structured](../formats/_index.md)
format, providing full fidelity of the original format and the ability
to reconstruct the original data without loss of information.

SuperDB data lakes provide an easy-to-use substrate for data discovery, preparation,
and transformation as well as serving as a queryable and searchable store
for super-structured data both for online and archive use cases.

<p id="status"></p>

> While `super` and its accompanying formats
> are production quality, the SuperDB lakehouse is still fairly early in development
> and alpha quality.
> That said, SuperDB lakehouses can be utilized quite effectively at small scale,
> or at larger scales when scripted automation
> is deployed to manage the lake's data layout via the
> [lake API](../lake/api.md).
>
> Enhanced scalability with self-tuning configuration is under development.


## The Lakehouse Model

A SuperDB lakehouse is a cloud-native arrangement of data, optimized for search,
analytics, ETL, data discovery, and data preparation
at scale based on data represented in accordance
with the [Super data model](../formats/data-model.md).

A lakehouse is organized into a collection of data pools forming a single
administrative domain.  The current implementation supports
ACID append and delete semantics at the commit level while
we have plans to support CRUD updates at the primary-key level
in the near future.

TODO: back off on github metaphor?

The semantics of a SuperDB lakehouse loosely follows the nomenclature and
design patterns of [`git`](https://git-scm.com/).  In this approach,
* a _lake_ is like a GitHub organization,
* a _pool_ is like a `git` repository,
* a _branch_ of a _pool_ is like a `git` branch,
* the _use_  command is like a `git checkout`, and
* the _load_ command is like a `git add/commit/push`.

A core theme of the SuperDB lakehouse design is _ergonomics_.  Given the Git metaphor,
our goal here is that the lake tooling be as easy and familiar as Git is
to a technical user.

Since SuperDB lakehouses are built around the Super data model,
getting different kinds of data into and out of a lake is easy.
There is no need to define schemas or tables and then fit
semi-structured data into schemas before loading data into a lake.
And because SuperDB supports a large family of formats and the load endpoint
automatically detects most formats, it's easy to just load data into a lake
without thinking about how to convert it into the right format.

### CLI-First Approach

The SuperDB project has taken a _CLI-first approach_ to designing and implementing
the system.  Any time a new piece of functionality is added to the lake,
it is first implemented as a `super db` command.  This is particularly convenient
for testing and continuous integration as well as providing intuitive,
bite-sized chunks for learning how the system works and how the different
components come together.

While the CLI-first approach provides these benefits,
all of the functionality is also exposed through [an API](../lake/api.md) to
a lake service.  Many use cases involve an application like
[SuperDB Desktop](https://zui.brimdata.io/) or a
programming environment like Python/Pandas interacting
with the service API in place of direct use with `super db`.

### Storage Layer

The lake storage model is designed to leverage modern cloud object stores
and separates compute from storage.

A lake is entirely defined by a collection of cloud objects stored
at a configured object-key prefix.  This prefix is called the _storage path_.
All of the meta-data describing the data pools, branches, commit history,
and so forth is stored as cloud objects inside of the lake.  There is no need
to set up and manage an auxiliary metadata store.

Data is arranged in a lake as a set of pools, which are comprised of one
or more branches, which consist of a sequence of data [commit objects](#commit-objects)
that point to cloud data objects.

Cloud objects and commits are immutable and named with globally unique IDs,
based on the [KSUIDs](https://github.com/segmentio/ksuid), and many
commands may reference various lake entities by their ID, e.g.,
* _Pool ID_ - the KSUID of a pool
* _Commit object ID_ - the KSUID of a commit object
* _Data object ID_ - the KSUID of a committed data object

Data is added and deleted from the lake only with new commits that
are implemented in a transactionally consistent fashion.  Thus, each
commit object (identified by its globally-unique ID) provides a completely
consistent view of an arbitrarily large amount of committed data
at a specific point in time.

While this commit model may sound heavyweight, excellent live ingest performance
can be achieved by micro-batching commits.

Because the lake represents all state transitions with immutable objects,
the caching of any cloud object (or byte ranges of cloud objects)
is easy and effective since a cached object is never invalid.
This design makes backup/restore, data migration, archive, and
replication easy to support and deploy.

The cloud objects that comprise a lake, e.g., data objects,
commit history, transaction journals, partial aggregations, etc.,
are stored as super-structured data, i.e., either as [row-based Super Binary (BSUP)](../formats/bsup.md)
or [Super Columnar (CSUP)](../formats/csup.md).
This makes introspection of the lake structure straightforward as many key
lake data structures can be queried with metadata queries and presented
to a client for further processing by downstream tooling.

The implementation also includes a storage abstraction that maps the cloud object
model onto a file system so that lakes can also be deployed on standard file systems.

### Command Personalities

The `super db` command provides a single command-line interface to SuperDB data lakes, but
different personalities are taken on by `super db` depending on the particular
sub-command executed and the [lake location](#locating-the-lake).

To this end, `super db` can take on one of three personalities:

* _Direct Access_ - When the lake is a storage path (`file` or `s3` URI),
then the `super db` commands (except for `serve`) all operate directly on the
lake located at that path.
* _Client Personality_ - When the lake is an HTTP or HTTPS URL, then the
lake is presumed to be a service endpoint and the client
commands are directed to the service managing the lake.
* _Server Personality_ - When the [`super db serve`](#serve) command is executed, then
the personality is always the server personality and the lake must be
a storage path.  This command initiates a continuous server process
that serves client requests for the lake at the configured storage path.

Note that a storage path on the file system may be specified either as
a fully qualified file URI of the form `file://` or be a standard
file system path, relative or absolute, e.g., `/lakes/test`.

Concurrent access to any lake storage, of course, preserves
data consistency.  You can run multiple `super db serve` processes while also
running any `super db` lake command all pointing at the same storage endpoint
and the lake's data footprint will always remain consistent as the endpoints
all adhere to the consistency semantics of the lake.

> Transactional data consistency is not fully implemented yet for
> the S3 endpoint so only single-node access to S3 is available right now,
> though support for multi-node access is forthcoming.
> For a shared file system, the close-to-open cache consistency
> semantics of [NFS](https://en.wikipedia.org/wiki/Network_File_System) should provide the > necessary consistency guarantees needed by
> the lake though this has not been tested.  Multi-process, single-node
> access to a local file system has been thoroughly tested and should be
> deemed reliable, i.e., you can run a direct-access instance of `super db` alongside
> a server instance of `super db` on the same file system and data consistency will
> be maintained.

### Locating the Lakehouse

At times you may want `super db` commands to access the same lake storage
used by other tools such as [SuperDB Desktop](https://zui.brimdata.io/). To help
enable this by default while allowing for separate lake storage when desired,
`super db` checks each of the following in order to attempt to locate an existing
lake.

1. The contents of the `-lake` option (if specified)
2. The contents of the `SUPER_DB_LAKE` environment variable (if defined)
3. A lake service running locally at `http://localhost:9867` (if a socket
   is listening at that port)
4. A `super` subdirectory below a path in the
   [`XDG_DATA_HOME`](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)
   environment variable (if defined)
5. A default file system location based on detected OS platform:
   - `%LOCALAPPDATA%\super` on Windows
   - `$HOME/.local/share/super` on Linux and macOS

### Data Pools

A lakehouse is made up of _data pools_, which are like "collections" in NoSQL
document stores.  Pools may have one or more branches and every pool always
has a branch called `main`.

A pool is created with the [`create` command](#create)
and a branch of a pool is created with the [`branch` command](#branch).

A pool name can be any valid UTF-8 string and is allocated a unique ID
when created.  The pool can be referred to by its name or by its ID.
A pool may be renamed but the unique ID is always fixed.

#### Commit Objects

Data is added into a pool in atomic units called _commit objects_.

Each commit object is assigned a global ID.
Similar to Git, commit objects are arranged into a tree and
represent the entire commit history of the lake.

> Technically speaking, Git can merge from multiple parents and thus
> Git commits form a directed acyclic graph instead of a tree;
> SuperDB does not currently support multiple parents in the commit object history.

A branch is simply a named pointer to a commit object in the lake
and like a pool, a branch name can be any valid UTF-8 string.
Consistent updates to a branch are made by writing a new commit object that
points to the previous tip of the branch and updating the branch to point at
the new commit object.  This update may be made with a transaction constraint
(e.g., requiring that the previous branch tip is the same as the
commit object's parent); if the constraint is violated, then the transaction
is aborted.

The _working branch_ of a pool may be selected on any command with the `-use` option
or may be persisted across commands with the [`use` command](#use) so that
`-use` does not have to be specified on each command-line.  For interactive
workflows, the `use` command is convenient but for automated workflows
in scripts, it is good practice to explicitly specify the branch in each
command invocation with the `-use` option.

#### Commitish

Many `super db` commands operate with respect to a commit object.
While commit objects are always referenceable by their commit ID, it is also convenient
to refer to the commit object at the tip of a branch.

The entity that represents either a commit ID or a branch is called a _commitish_.
A commitish is always relative to the pool and has the form:
* `<pool>@<id>` or
* `<pool>@<branch>`

where `<pool>` is a pool name or pool ID, `<id>` is a commit object ID,
and `<branch>` is a branch name.

In particular, the working branch set by the [`use` command](#use) is a commitish.

A commitish may be abbreviated in several ways where the missing detail is
obtained from the working-branch commitish, e.g.,
* `<pool>` - When just a pool name is given, then the commitish is assumed to be
`<pool>@main`.
* `@<id>` or `<id>`- When an ID is given (optionally with the `@` prefix), then the commitish is assumed to be `<pool>@<id>` where `<pool>` is obtained from the working-branch commitish.
* `@<branch>` - When a branch name is given with the `@` prefix, then the commitish is assumed to be `<pool>@<id>` where `<pool>` is obtained from the working-branch commitish.

An argument to a command that takes a commit object is called a _commitish_
since it can be expressed as a branch or as a commit ID.

#### Pool Key

Each data pool is organized according to its configured _pool key_,
which is the sort key for all data stored in the lake.  Different data pools
can have different pool keys but all of the data in a pool must have the same
pool key.

As pool data is often comprised of [records](../formats/data-model.md#21-record) (analogous to JSON objects),
the pool key is typically a field of the stored records.
When pool data is not structured as records/objects (e.g., scalar or arrays or other
non-record types), then the pool key would typically be configured
as the [special value `this`](../language/pipeline-model.md#the-special-value-this).

Data can be efficiently scanned if a query has a filter operating on the pool
key.  For example, on a pool with pool key `ts`, the query `ts == 100`
will be optimized to scan only the data objects where the value `100` could be
present.

> The pool key will also serve as the primary key for the forthcoming
> CRUD semantics.

A pool also has a configured sort order, either ascending or descending
and data is organized in the pool in accordance with this order.
Data scans may be either ascending or descending, and scans that
follow the configured order are generally more efficient than
scans that run in the opposing order.

Scans may also be range-limited but unordered.

Any data loaded into a pool that lacks the pool key is presumed
to have a null value with regard to range scans.  If large amounts
of such "keyless data" are loaded into a pool, the ability to
optimize scans over such data is impaired.

### Time Travel

Because commits are transactional and immutable, a query
sees its entire data scan as a fixed "snapshot" with respect to the
commit history.  In fact, the [`from` operator](../language/operators/from.md)
allows a commit object to be specified with the `@` suffix to a
pool reference, e.g.,
```
super db query 'from logs@1tRxi7zjT7oKxCBwwZ0rbaiLRxb | ...'
```
In this way, a query can time-travel through the commit history.  As long as the
underlying data has not been deleted, arbitrarily old snapshots of the
lake can be easily queried.

If a writer commits data after or while a reader is scanning, then the reader
does not see the new data since it's scanning the snapshot that existed
before these new writes occurred.

Also, arbitrary metadata can be committed to the log [as described below](#load),
e.g., to associate derived analytics to a specific
journal commit point potentially across different data pools in
a transactionally consistent fashion.

While time travel through commit history provides one means to explore
past snapshots of the commit history, another means is to use a timestamp.
Because the entire history of branch updates is stored in a transaction journal
and each entry contains a timestamp, branch references can be easily
navigated by time.  For example, a list of branches of a pool's past
can be created by scanning the internal "pools log" and stopping at the largest
timestamp less than or equal to the desired timestamp.  Then using that
historical snapshot of the pools, a branch can be located within the pool
using that pool's "branches log" in a similar fashion, then its corresponding
commit object can be used to construct the data of that branch at that
past point in time.

> Time travel using timestamps is a forthcoming feature.

## `super db` Commands

While `super db` is itself a sub-command of [`super`](super.md), it invokes
a large number of interrelated sub-commands, similar to the
[`docker`](https://docs.docker.com/engine/reference/commandline/cli/)
or [`kubectl`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands)
commands.

The following sections describe each of the available commands and highlight
some key options.  Built-in help shows the commands and their options:

* `super db -h` with no args displays a list of `super db` commands.
* `super db command -h`, where `command` is a sub-command, displays help
for that sub-command.
* `super db command sub-command -h` displays help for a sub-command of a
sub-command and so forth.

By default, commands that display lake metadata (e.g., [`log`](#log) or
[`ls`](#ls)) use the human-readable [lake metadata output](super.md#superdb-data-lake-metadata-output)
format.  However, the `-f` option can be used to specify any supported
[output format](super.md#output-formats).

### Auth
```
super db auth login|logout|method|verify
```
Access to a lake can be secured with [Auth0 authentication](https://auth0.com/).
A [guide](../integrations/zed-lake-auth/index.md) is available with example configurations.
Please reach out to us on our [community Slack](https://www.brimdata.io/join-slack/)
if you have feedback on your experience or need additional help.

### Branch
```
super db branch [options] [name]
```
The `branch` command creates a branch with the name `name` that points
to the tip of the working branch or, if the `name` argument is not provided,
lists the existing branches of the selected pool.

For example, this branch command
```
super db branch -use logs@main staging
```
creates a new branch called "staging" in pool "logs", which points to
the same commit object as the "main" branch.  Once created, commits
to the "staging" branch will be added to the commit history without
affecting the "main" branch and each branch can be queried independently
at any time.

Supposing the `main` branch of `logs` was already the working branch,
then you could create the new branch called "staging" by simply saying
```
super db branch staging
```
Likewise, you can delete a branch with `-d`:
```
super db branch -d staging
```
and list the branches as follows:
```
super db branch
```

### Create
```
super db create [-orderby key[,key...][:asc|:desc]] <name>
```
The `create` command creates a new data pool with the given name,
which may be any valid UTF-8 string.

The `-orderby` option indicates the [pool key](#pool-key) that is used to sort
the data in lake, which may be in ascending or descending order.

If a pool key is not specified, then it defaults to
the [special value `this`](../language/pipeline-model.md#the-special-value-this).

A newly created pool is initialized with a branch called `main`.

> Lakes can be used without thinking about branches.  When referencing a pool without
> a branch, the tooling presumes the "main" branch as the default, and everything
> can be done on main without having to think about branching.

### Delete
```
super db delete [options] <id> [<id>...]
super db delete [options] -where <filter>
```
The `delete` command removes one or more data objects indicated by their ID from a pool.
This command
simply removes the data from the branch without actually deleting the
underlying data objects thereby allowing time travel to work in the face
of deletes.  Permanent deletion of underlying data objects is handled by the
separate [`vacuum`](#vacuum) command.

If the `-where` flag is specified, delete will remove all values for which the
provided filter expression is true.  The value provided to `-where` must be a
single filter expression, e.g.:

```
super db delete -where 'ts > 2022-10-05T17:20:00Z and ts < 2022-10-05T17:21:00Z'
```

### Drop
```
super db drop [options] <name>|<id>
```
The `drop` command deletes a pool and all of its constituent data.
As this is a DANGER ZONE command, you must confirm that you want to delete
the pool to proceed.  The `-f` option can be used to force the deletion
without confirmation.

### Init
```
super db init [path]
```
A new lake is initialized with the `init` command.  The `path` argument
is a [storage path](#storage-layer) and is optional.  If not present, the path
is [determined automatically](#locating-the-lake).

If the lake already exists, `init` reports an error and does nothing.

Otherwise, the `init` command writes the initial cloud objects to the
storage path to create a new, empty lake at the specified path.

### Load
```
super db load [options] input [input ...]
```
The `load` command commits new data to a branch of a pool.

Run `super db load -h` for a list of command-line options.

Note that there is no need to define a schema or insert data into
a "table" as all super-structured data is _self describing_ and can be queried in a
schema-agnostic fashion.  Data of any _shape_ can be stored in any pool
and arbitrary data _shapes_ can coexist side by side.

As with [`super`](super.md),
the [input arguments](super.md#usage) can be in
any [supported format](super.md#input-formats) and
the input format is auto-detected if `-i` is not provided.  Likewise,
the inputs may be URLs, in which case, the `load` command streams
the data from a Web server or [S3](../integrations/amazon-s3.md) and into the lake.

When data is loaded, it is broken up into objects of a target size determined
by the pool's `threshold` parameter (which defaults to 500MiB but can be configured
when the pool is created).  Each object is sorted by the [pool key](#pool-key) but
a sequence of objects is not guaranteed to be globally sorted.  When lots
of small or unsorted commits occur, data can be fragmented.  The performance
impact of fragmentation can be eliminated by regularly [compacting](#manage)
pools.

For example, this command
```
super db load sample1.json sample2.bsup sample3.sup
```
loads files of varying formats in a single commit to the working branch.

An alternative branch may be specified with a branch reference with the
`-use` option, i.e., `<pool>@<branch>`.  Supposing a branch
called `live` existed, data can be committed into this branch as follows:
```
super db load -use logs@live sample.bsup
```
Or, as mentioned above, you can set the default branch for the load command
via [`use`](#use):
```
super db use logs@live
super db load sample.bsup
```
During a `load` operation, a commit is broken out into units called _data objects_
where a target object size is configured into the pool,
typically 100MB-1GB.  The records within each object are sorted by the pool key.
A data object is presumed by the implementation
to fit into the memory of an intake worker node
so that such a sort can be trivially accomplished.

Data added to a pool can arrive in any order with respect to the pool key.
While each object is sorted before it is written,
the collection of objects is generally not sorted.

Each load operation creates a single [commit object](#commit-objects), which includes:
* an author and message string,
* a timestamp computed by the server, and
* an optional metadata field of any type expressed as a Super (SUP) value.
This data has the type signature:
```
{
    author: string,
    date: time,
    message: string,
    meta: <any>
}
```
where `<any>` is the type of any optionally attached metadata .
For example, this command sets the `author` and `message` fields:
```
super db load -user user@example.com -message "new version of prod dataset" ...
```
If these fields are not specified, then the system will fill them in
with the user obtained from the session and a message that is descriptive
of the action.

The `date` field here is used by the lake system to do [time travel](#time-travel)
through the branch and pool history, allowing you to see the state of
branches at any time in their commit history.

Arbitrary metadata expressed as any [SUP value](../formats/sup.md)
may be attached to a commit via the `-meta` flag.  This allows an application
or user to transactionally commit metadata alongside committed data for any
purpose.  This approach allows external applications to implement arbitrary
data provenance and audit capabilities by embedding custom metadata in the
commit history.

Since commit objects are stored as super-structured data, the metadata can easily be
queried by running the `log -f bsup` to retrieve the log in BSUP format,
for example, and using [`super`](super.md) to pull the metadata out
as in:
```
super db log -f bsup | super -c 'has(meta) | yield {id,meta}' -
```

### Log
```
super db log [options] [commitish]
```
The `log` command, like `git log`, displays a history of the [commit objects](#commit-objects)
starting from any commit, expressed as a [commitish](#commitish).  If no argument is
given, the tip of the working branch is used.

Run `super db log -h` for a list of command-line options.

To understand the log contents, the `load` operation is actually
decomposed into two steps under the covers:
an "add" step stores one or more
new immutable data objects in the lake and a "commit" step
materializes the objects into a branch with an ACID transaction.
This updates the branch pointer to point at a new commit object
referencing the data objects where the new commit object's parent
points at the branch's previous commit object, thus forming a path
through the object tree.

The `log` command prints the commit ID of each commit object in that path
from the current pointer back through history to the first commit object.

A commit object includes
an optional author and message, along with a required timestamp,
that is stored in the commit journal for reference.  These values may
be specified as options to the [`load`](#load) command, and are also available in the
[lake API](../lake/api.md) for automation.

> The branchlog meta-query source is not yet implemented.

### Ls
```
super db ls [options] [pool]
```
The `ls` command lists pools in a lake or branches in a pool.

By default, all pools in the lake are listed along with each pool's unique ID
and [pool key](#pool-key) configuration.

If a pool name or pool ID is given, then the pool's branches are listed along
with the ID of their commit object, which points at the tip of each branch.

### Manage
```
super db manage [options]
```
The `manage` command performs maintenance tasks on a lake.

Currently the only supported task is _compaction_, which reduces fragmentation
by reading data objects in a pool and writing their contents back to large,
non-overlapping objects.

If the `-monitor` option is specified and the lake is [located](#locating-the-lake)
via network connection, `super db manage` will run continuously and perform updates
as needed.  By default a check is performed once per minute to determine if
updates are necessary.  The `-interval` option may be used to specify an
alternate check frequency in [duration format](../formats/sup.md#23-primitive-values).

If `-monitor` is not specified, a single maintenance pass is performed on the
lake.

By default, maintenance tasks are performed on all pools in the lake.  The
`-pool` option may be specified one or more times to limit maintenance tasks
to a subset of pools listed by name.

The output from `manage` provides a per-pool summary of the maintenance
performed, including a count of `objects_compacted`.

As an alternative to running `manage` as a separate command, the `-manage`
option is also available on the [`serve`](#serve) command to have maintenance
tasks run at the specified interval by the service process.

### Merge

Data is merged from one branch into another with the `merge` command, e.g.,
```
super db merge -use logs@updates main
```
where the `updates` branch is being merged into the `main` branch
within the `logs` pool.

A merge operation finds a common ancestor in the commit history then
computes the set of changes needed for the target branch to reflect the
data additions and deletions in the source branch.
While the merge operation is performed, data can still be written concurrently
to both branches and queries performed and everything remains transactionally
consistent.  Newly written data remains in the
branch while all of the data present at merge initiation is merged into the
parent.

This Git-like behavior for a data lake provides a clean solution to
the live ingest problem.
For example, data can be continuously ingested into a branch of `main` called `live`
and orchestration logic can periodically merge updates from branch `live` to
branch `main`, possibly [compacting](#manage) data after the merge
according to configured policies and logic.

### Query
```
super db query [options] <query>
```
The `query` command runs a [SuperSQL](../language/_index.md) query with data from a lake as input.
A query typically begins with a [`from` operator](../language/operators/from.md)
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

We can narrow the span of the query by specifying a filter on the [pool key](#pool-key):
```
super db query 'from logs | ts >= 2018-03-24T17:36:30.090766Z and ts <= 2018-03-24T17:36:30.090758Z'
```
Filters on pool keys are efficiently implemented as the data is laid out
according to the pool key and seek indexes keyed by the pool key
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
By default, the `query` command scans pool data in pool-key order though
the query optimizer may, in general, reorder the scan to optimize searches,
aggregations, and joins.
An order hint can be supplied to the `query` command to indicate to
the optimizer the desired processing order, but in general, [`sort` operators](../language/operators/sort.md)
should be used to guarantee any particular sort order.

Arbitrarily complex queries can be executed over the lake in this fashion
and the planner can utilize cloud resources to parallelize and scale the
query over many parallel workers that simultaneously access the lake data in
shared cloud storage (while also accessing locally- or cluster-cached copies of data).

#### Meta-queries

Commit history, metadata about data objects, lake and pool configuration,
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

### Rename
```
super db rename <existing> <new-name>
```
The `rename` command assigns a new name `<new-name>` to an existing
pool `<existing>`, which may be referenced by its ID or its previous name.

### Serve
```
super db serve [options]
```
The `serve` command implements the [server personality](#command-personalities) to service requests
from instances of the client personality.
It listens for [lake API](../lake/api.md) requests on the interface and port
specified by the `-l` option, executes the requests, and returns results.

The `-log.level` option controls log verbosity.  Available levels, ordered
from most to least verbose, are `debug`, `info` (the default), `warn`,
`error`, `dpanic`, `panic`, and `fatal`.  If the volume of logging output at
the default `info` level seems too excessive for production use, `warn` level
is recommended.

The `-manage` option enables the running of the same maintenance tasks
normally performed via the separate [`manage`](#manage) command.

### Use
```
super db use [<commitish>]
```
The `use` command sets the working branch to the indicated commitish.
When run with no argument, it displays the working branch and [lake](#locating-the-lake).

For example,
```
super db use logs
```
provides a "pool-only" commitish that sets the working branch to `logs@main`.

If a `@branch` or commit ID are given without a pool prefix, then the pool of
the commitish previously in use is presumed.  For example, if you are on
`logs@main` then run this command:
```
super db use @test
```
then the working branch is set to `logs@test`.

To specify a branch in another pool, simply prepend
the pool name to the desired branch:
```
super db use otherpool@otherbranch
```
This command stores the working branch in `$HOME/.super_head`.

### Vacuum
```
super db vacuum [options]
```

The `vacuum` command permanently removes underlying data objects that have
previously been subject to a [`delete`](#delete) operation.  As this is a
DANGER ZONE command, you must confirm that you want to remove
the objects to proceed.  The `-f` option can be used to force removal
without confirmation.  The `-dryrun` option may also be used to see a summary
of how many objects would be removed by a `vacuum` but without removing them.
