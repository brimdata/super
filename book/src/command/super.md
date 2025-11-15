## Command

&emsp; **super** &mdash; invoke or manage SuperDB

## Synopsis
```
super [ -c query ] [ options ] [ file ... ]
super [ options ] <sub-command> ...
```
## Sub-commands

* [compile](compile.md)
* [db](db.md)
* [dev](dev.md)

## Options

> **TODO: link these short-hand flag descriptions to longer form descriptions**

* [Output Options](output-options.md)
* `-aggmem` maximum memory used per aggregate function value in MiB, MB, etc
* `-c` [SuperSQL](../super-sql/intro.md) query to execute
* `-csv.delim` CSV field delimiter
* `-e` stop upon input errors
* `-fusemem` maximum memory used by fuse in MiB, MB, etc
* `-h` display help
* `-help` display help
* `-hidden` show hidden options
* `-i` format of input data
* `-I` source file containing query text
* `-q` don't display warnings
* `-sortmem` maximum memory used by sort in MiB, MB, etc
* `-stats` display search stats on stderr
* `-version` print version and exit

## Description

`super` is the command-line tool for interacting with and managing SuperDB
and is organized as a hierarchy of sub-commands similar to 
[`docker`](https://docs.docker.com/engine/reference/commandline/cli/)
or [`kubectl`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands).

For built-in command help and a listing of all available options,
simply run `super` without any arguments.

When invoked at the top level without a sub-command, `super` executes the
SuperDB query engine detached from the database storage layer
where the data inputs may be files, HTTP APIs, S3 cloud objects, or standard input.

Optional [SuperSQL](../super-sql/intro.md) query text may be provided with
the `-c` argument.  If no query is provided, the inputs are scanned
and output is produced in accordance with `-f` to specify a serialization format
and `-o` to specified an optional output (file or directory).

When invoked using the [db](db.md) sub-command, `super` interacts with
an underlying SuperDB database.

The [dev](dev.md) sub-command provides dev tooling for the advanced users or
developers of SuperDB while the [compile](compile.md) command allows detailed
interactions with various stages of the query compiler.

### Input

When run detached from a database, `super` executes a query over inputs
external to the database including
* file system paths,
* standard input, or
* HTTP, HTTPS, or S3 URLs.

These inputs may be specified with the  operator
within the query text or via the file arguments (including stdin) to the command.

Command-line paths are treated as if a
[from](../super-sql/operators/from.md) operator precedes
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
When multiple input files are specified, they are processed in the order given as
if the data were provided by a single, concatenated `FROM` clause.

If no input is specified,
the query is fed a single `null` value analogous to SQL's default
input of a single empty row of an unnamed table.

This provides a convenient means to explore examples or run in a
"calculator mode", e.g.,
```mdtest-command
super -s -c '1+1'
```
which is [shorthand](../super-sql/operators/intro.md#shortcuts)
for `values 1+1`, emits
```mdtest-output
2
```

`super` currently supports the following input formats:

|  Option   | Auto | Extension | Specification                            |
|-----------|------|-----------|------------------------------------------|
| `arrows`  |  yes | `.arrows` | [Arrow IPC Stream Format](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format) |
| `bsup`    |  yes | `.bsup` | [BSUP](../formats/bsup.md) |
| `csup`    |  yes | `.csup` | [CSUP](../formats/csup.md) |
| `csv`     |  yes | `.csv` | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `json`    |  yes | `.json` | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
| `jsup`   |  yes | `.jsup` | [Super over JSON (JSUP)](../formats/jsup.md) |
| `line`    |  no  | n/a | One string value per input line |
| `parquet` |  yes | `.parquet` | [Apache Parquet](https://github.com/apache/parquet-format) |
| `sup`     |  yes | `.sup` | [SUP](../formats/sup.md) |
| `tsv`     |  yes | `.tsv` | [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values) |
| `zeek`    |  yes | `.zeek` | [Zeek Logs](https://docs.zeek.org/en/master/logs/index.html) |

> _Best performance is achieved when operating on data in binary columnar formats
> such as [CSUP](../formats/csup.md),
> [Parquet](https://github.com/apache/parquet-format), or
> [Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format)._

For most types of data, the input format is automatically detected as indicated
by a "yes" in the "Auto" column above.
If multiple files are specified, each file format is determined independently
so you can mix and match input types.

Formats without auto-detection require the `-i` option.

#### Hard-wired Input Format

The input format is specified with the `-i` flag.

When `-i` is specified, all of the inputs on the command-line must be
in the indicated format.

#### Auto-detection

> **TODO: clarify any format inference based on file extension.**

Without `-i`, `super` uses _auto-detection to infer each input's format.

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

#### JSON vs SUP Autodetection

Since [SUP](../formats/sup.md) is a superset of plain JSON, `super` must be careful how it distinguishes the two cases when performing auto-inference.
While you can always clarify your intent
via `-i sup` or `-i json`, `super` attempts to "just do the right thing"
when you run it with SUP vs. plain JSON.

While `super` can parse any JSON using its built-in SUP parser this is typically
not desirable because (1) the SUP parser is not particularly performant and
(2) all JSON numbers are floating point but the SUP parser will parse as
JSON any number that appears without a decimal point as an integer type.

> Note that `super` is not particularly performant for SUP because it is intended
> as a human-readable format while the high-performance
> columnar [CSUP](../formats/csup.md) is semantically equivalent but much more efficient.
> The design intent is that these efficient binary formats should be used in
> use cases where performance matter and SUP is typically used only when
> data needs to be human-readable in interactive settings or in automated tests.

To this end, `super` uses a heuristic to select between SUP and plain JSON when the
`-i` option is not specified. Specifically, plain JSON is selected when the first values
of the input are parsable as valid JSON and includes a JSON object either
as an outer object or as a value nested somewhere within a JSON array.

This heuristic almost always works in practice because SUP records
typically omit quotes around field names.

### Output

> **TODO: make CSUP not BSUP the default output format when not a terminal.**

Output is written to one or more files or to standard output in the
format specified `-f`, which if omitted, defaults to [SUP](../formats/sup.md).

Output is sent to standard output unless an output file is
specified with -o.

When writing to stdout and stdout is a terminal, the default
output format is [SUP](../formats/sup.md).
Otherwise, the default format is [CSUP](../formats/csup.md).
In either case, the default may be overridden with `-f`, `-s`, or `-S`.

Some output formats like Parquet are based on
schemas and require all data in the output to conform to the same
schema.  To handle this, you can either fuse the data into a union
of all the record types present (presuming all the output values
are records) or you can specify the -split flag to indicate a
destination directory for separate output files for each output
type.  This flag may be used in combination with -o, which provides
the prefix for the file path, e.g.,

super -f parquet -split out -o example-output input.bsup



The query text may include source files using -I, which is
particularly convenient when a large, complex query spans multiple
lines.  In this case, these source files are concatenated together
along with the command-line query text in the order appearing on
the command line.  Any error messages are properly collated to the
included file in which they occurred.

The runtime processes input natively as super-structured data so
if you intend to run many queries over the same data, you will see
substantial performance gains by converting your data to the Super
Binary format, e.g.,

super -f bsup input.any > fast.bsup

super -c <query> fast.bsup

## XXX

==OUTPUT==

If no query is specified with `-c`, the inputs are scanned without modification
and output in the desired format as [described below](#input-formats),
providing a convenient means to convert files from one format to another, e.g.,
```
super -f arrows file1.json file2.parquet file3.csv > file-combined.arrows
```



## Data Formats

`super` supports a number of [input](#input-formats) and [output](#output-formats) formats, but the
[SUP](../formats/sup.md),
[BSUP](../formats/bsup.md), and
[CSUP](../formats/csup.md) formats tend to be the most versatile and
easy to work with.



Unless the `-i` option specifies a specific input format,
each input's format is [automatically inferred](#auto-detection)
and each input is scanned
in the order appearing on the command line forming the input stream.

### Input Formats




### Output Formats

`super` currently supports the following output formats:

|  Option   | Specification                            |
|-----------|------------------------------------------|
| `arrows`  | [Arrow IPC Stream Format](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format) |
| `bsup`    | [BSUP](../formats/bsup.md) |
| `csup`    | [CSUP](../formats/csup.md) |
| `csv`     | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `db`      | [SuperDB Database Metadata Output](#superdb-data-lake-metadata-output) |
| `json`    | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
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
{x:1,s:null::string}
{x:null::int64,s:"hello"}
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

#### SuperDB Database Metadata Output

TODO: change this to dbmeta

The `db` format is used to pretty-print lake metadata, such as in
[`super db` sub-command](super-db.md) outputs.  Because it's `super db`'s default output format,
it's rare to request it explicitly via `-f`.  However, since it's possible for
`super db` to [generate output in any supported format](super-db.md#super-db-commands),
the `db` format is useful to reverse this.

For example, imagine you'd executed a [meta-query](super-db.md#meta-queries) via
`super db query -S "from :pools"` and saved the output in this file `pools.sup`.

```mdtest-input pools.sup
{
    ts: 2024-07-19T19:28:22.893089Z,
    name: "MyPool",
    id: 0x132870564f00de22d252b3438c656691c87842c2::=ksuid.KSUID,
    layout: {
        order: "desc"::=order.Which,
        keys: [
            [
                "ts"
            ]::=field.Path
        ]::=field.List
    }::=order.SortKey,
    seek_stride: 65536,
    threshold: 524288000
}::=pools.Config
```

Using `super -f db`, this can be rendered in the same pretty-printed form as it
would have originally appeared in the output of `super db ls`, e.g.,

```mdtest-command
super -f db pools.sup
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
