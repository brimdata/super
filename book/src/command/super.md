# Command

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

The query text may originate in files using one or more `-I` arguments.
In this case, these source files are concatenated together in order and prepended
to any `-c` query text.  `-I` may be used without `-c`.

When invoked using the [db](db.md) sub-command, `super` interacts with
an underlying SuperDB database.

The [dev](dev.md) sub-command provides dev tooling for the advanced users or
developers of SuperDB while the [compile](compile.md) command allows detailed
interactions with various stages of the query compiler.

### Supported Formats

|  Option   | Auto | Extension | Specification                            |
|-----------|------|-----------|------------------------------------------|
| `arrows`  |  yes | `.arrows` | [Arrow IPC Stream Format](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format) |
| `bsup`    |  yes | `.bsup` | [BSUP](../formats/bsup.md) |
| `csup`    |  yes | `.csup` | [CSUP](../formats/csup.md) |
| `csv`     |  yes | `.csv` | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `json`    |  yes | `.json` | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
| `jsup`   |  yes | `.jsup` | [Super over JSON (JSUP)](../formats/jsup.md) |
| `line`    |  no  | n/a | One text value per line |
| `parquet` |  yes | `.parquet` | [Apache Parquet](https://github.com/apache/parquet-format) |
| `sup`     |  yes | `.sup` | [SUP](../formats/sup.md) |
| `tsv`     |  yes | `.tsv` | [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values) |
| `zeek`    |  yes | `.zeek` | [Zeek Logs](https://docs.zeek.org/en/master/logs/index.html) |

>[!NOTE]
> Best performance is achieved when operating on data in binary columnar formats
> such as [CSUP](../formats/csup.md),
> [Parquet](https://github.com/apache/parquet-format), or
> [Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format).

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
input of a single empty row of an unnamed table.  This provides a convenient means
to run standalone examples or compute results like a calculator, e.g.,
```mdtest-command
super -s -c '1+1'
```
is [shorthand](../super-sql/operators/intro.md#shortcuts)
for `values 1+1` and emits
```mdtest-output
2
```

#### Format Detection

In general, `super` _just works_ when it comes to automatically inferring
the data formats of its inputs.

For files with a well known extension (like `.json`, `.parquet`, `.sup` etc.),
the format is implied by the extension.

For standard input or files without a recognizable extension, `super` attempts
to detect the format by reading and parsing some of the data.

To override these format inference heuristics, `-i` may be used to specify
the input formats of command-line files or the `(format)` option of a data source
specified in a [from](../super-sql/operators/from.md) operator.

When `-i` is used, all of the input files must have the same format.
Without `-i`, each file format is determined independently so you can
mix and match input formats.

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
Note that the `line` format cannot be automatically detected and
requires `-i` or `(format line)` for reading.

> **TODO: Parquet and CSUP require a seekable input and cannot be operated upon
> when read on standard input.
> It seems like this should change given the pipe-able nature of super and
> the desire to make CSUP be the default output to a non-terminal output.**

### Output

> **TODO: make CSUP not BSUP the default output format when not a terminal.**

Output is written to standard output by default or, if `-o` is specified,
to the indicated file or directory.

When writing to stdout and stdout is a terminal, the default
output format is [SUP](../formats/sup.md).
Otherwise, the default format is [CSUP](../formats/csup.md).
These defaults may be overridden with `-f`, `-s`, or `-S`.

Since SUP is a common format choice for interactive use,
the `-s` flag is shorthand for `-f sup`.
Also, `-S` is a shortcut for `-f sup` with `-pretty 2` as
[described below](#pretty-printing).

And since plain JSON is another common format choice, the `-j` flag
is a shortcut for `-f json` and `-J` is a shortcut for pretty-printing JSON.

>[!NOTE]
> Having the default output format dependent on the terminal status
> causes an occasional surprise
> (e.g., forgetting `-f` or `-s` in a scripted test that works fine on the
> command line but fails in CI), this avoids problematic performance where a
> data pipeline deployed to product accidentally uses SUP instead of CSUP.
> Since `super` gracefully handles any input, this would be hard to detect.
> Alternatively, making CSUP the default would cause much annoyance when
> binary data is written to the terminal.

If no query is specified with `-c`, the inputs are scanned without modification
and output in the specified format
providing a convenient means to convert files from one format to another, e.g.,
```
super -f arrows -o  out.arrows file1.json file2.parquet file3.csv
```

#### Pretty Printing

SUP and plain JSON text may be "pretty printed" with the `-pretty` option, which takes
the number of spaces to use for indentation.  As this is a common option,
the `-S` option is a shortcut for `-f sup -pretty 2` and `-J` is a shortcut
for `-f json -pretty 2`.

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
echo '{a:{b:1,c:[1,2]},d:"foo"}' | super -f sup -pretty 4 -
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

#### Pipeline-friendly Formats

Though it's a compressed format, CSUP and BSUP data is self-describing and
stream-oriented and thus is pipeline friendly.

Since data is self-describing you can simply take super-structured output
of one command and pipe it to the input of another.  It doesn't matter if the value
sequence is scalars, complex types, or records.  There is no need to declare
or register schemas or "protos" with the downstream entities.

In particular, super-structured data can simply be concatenated together, e.g.,
```mdtest-command
super -f bsup -c 'values 1, [1,2,3]' > a.bsup
super -f bsup -c "values {s:'hello'}, {s:'world'}" > b.bsup
cat a.bsup b.bsup | super -s -
```
produces
```mdtest-output
1
[1,2,3]
{s:"hello"}
{s:"world"}
```

#### Schema-rigid Outputs

Certain data formats like [Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format)
and [Parquet](https://github.com/apache/parquet-format) are _schema rigid_
in the sense that they require a schema to be defined before
values can be written into the file and all the values in the file
must conform to this schema.

SuperDB, however, has a fine-grained type system instead of schemas such that a sequence
of data values is completely self-describing and may be heterogeneous in nature.
This creates a challenge converting the type-flexible super-structured data formats to a schema-rigid format like Arrow and Parquet.

For example, this seemingly simple conversion:
```mdtest-command fails
echo '{x:1}{s:"hello"}' | super -o out.parquet -f parquet -
```
causes this error
```mdtest-output
parquetio: encountered multiple types (consider 'fuse'): {x:int64} and {s:string}
```

To write heterogeneous data to a schema-based file format, you must
convert the data to a monolithic type.  To handle this,
you can either [fuse](../super-sql/operators/fuse.md)
the data into a single fused type or you can specify
the `-split` flag to indicate a destination directory that receives
a separate output file for each output type.

#### Fused Data

The [fuse](../super-sql/operators/fuse.md) operator uses
[type fusion](../super-sql/type-fusion.md) to merge different record
types into a blended type, e.g.,
```mdtest-command
echo '{x:1}{s:"hello"}' | super -o out.parquet -f parquet -c fuse -
super -s out.parquet
```
which produces
```mdtest-output
{x:1,s:null::string}
{x:null::int64,s:"hello"}
```
The downside of this approach is that the data muts be changed (by inserting nulls)
to conform to a single type.

Also, data fusion can sometimes involve sum types that are not
representable in a format like Parquet.  While a bit cumbersome,
you could write a query that adjusts the output be renaming columns
so that heterogenous data column types are avoided.   This modified
data could then be fused without sum types and output to Parquet.

#### Splitting Schemas

An alternative approach to the schema-rigid limitation of Arrow and
Parquet is to create a separate file for each schema.

`super` can do this too with its `-split` option, which specifies a path
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

#### SuperDB Database Metadata Output

> **TODO: We should get rid of this.  Or document it as an internal format.
> It's not a format that people should rely upon.**

The `db` format is used to pretty-print lake metadata, such as in
[`super db` sub-command](db.md) outputs.  Because it's `super db`'s default output format,
it's rare to request it explicitly via `-f`.  However, since it's possible for
`super db` to generate output in any supported format,
the `db` format is useful to reverse this.

For example, imagine you'd executed a [meta-query](db-query.md#meta-queries) via
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

### Line Format

The `line` format is convenient for interacting with other Unix-style tooling that
produces text input and output a line at a time.

When `-i line` is specified as the input format, data is read a line as a
[string](../super-sql/types/string.md) type.

When `-f line` is specified as the output format, each value is formatted
a line at a time.  String values are printed as is with otherwise escaped
values formatted as their native character in the output, e.g.,

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
Because embedded newlines create multi-lined output with `-i line`, this mode can
alter the sequence of values, e.g.,
```
super -c "values 'foo\nbar' | count()"
```
results in `1` but
```
super -f line -c "values 'foo\nbar'" | super -i line -c "count()" -
```
results in `2`.

## Debugging

> **TODO: this belongs in the super-sql section.  We can link to it.**

If you are ever stumped about how the `super` compiler is parsing your query,
you can always run `super -C` to compile and display your query in canonical form
without running it.
This can be especially handy when you are learning the language and its
[shortcuts](../super-sql/operators/intro.md#shortcuts).

For example, this query
```mdtest-command
super -C -c 'has(foo)'
```
is an implied [`where` operator](../super-sql/operators/where.md), which matches values
that have a field `foo`, i.e.,
```mdtest-output
where has(foo)
```
while this query
```mdtest-command
super -C -c 'a:=x+1'
```
is an implied [`put` operator](../super-sql/operators/put.md), which creates a new field `a`
with the value `x+1`, i.e.,
```mdtest-output
put a:=x+1
```

## Errors

> **TODO: this belongs in the super-sql section.  We can link to it.**
> **TODO: document compile-time errors and reference type checking.**

Fatal errors like "file not found" or "file system full" are reported
as soon as they happen and cause the `super` process to exit.

On the other hand,
runtime errors resulting from the query itself
do not halt execution.  Instead, these error conditions produce
[first-class errors](../super-sql/types/error.md)
in the data output stream interleaved with any valid results.
Such errors are easily queried with the
[`is_error` function](../super-sql/functions/errors/is_error.md).

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
