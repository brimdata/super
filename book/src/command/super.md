# super

The `super` command is invoked either by itself:
```
super [ -c query ] [ options ] [ file ... ]
```
or with a sub-command:
```
super [ options ] <sub-command> ...
```
The sub-commands include:
* [super compile](compile.md)
* [super db](db.md)
* [super dev](dev.md)

When invoked at the top level without a sub-command, `super` executes the
SuperDB query engine detached from the database storage layer
where the data inputs may be files, HTTP APIs, S3 cloud objects, or standard input.

For built-in command help and a listing of all available options,
simply run `super` without any arguments.

## Options

When running `super` without a sub-command, the command-line options
include:

* `-aggmem` maximum memory used per aggregate function value in MiB, MB, etc
* `-bsup.readmax` maximum Super Binary read buffer size in MiB, MB, etc.
* `-bsup.readsize` target Super Binary read buffer size in MiB, MB, etc.
* `-bsup.threads` number of Super Binary read threads
* `-bsup.validate` validate format when reading Super Binary
* `-c` [SuperSQL](../super-sql/intro.md) query to execute (may be used multiple times)
* `-csv.delim` CSV field delimiter
* `-e` stop upon input errors
* `-fusemem` maximum memory used by fuse in MiB, MB, etc
* `-h` display help
* `-help` display help
* `-hidden` show hidden options
* `-i` format of input data
* `-I` source file containing query text (may be used multiple times)
* `-q` don't display warnings
* `-sortmem` maximum memory used by sort in MiB, MB, etc
* `-stats` display search stats on stderr
* `-version` print version and exit
* any [output option](output.md#options)

An optional [SuperSQL](../super-sql/intro.md)
query is comprised of text specified by `-c` and source files
specified by `-I`.  Both `-c` and `-I` may appear multiple times and the
query text is concatenated in left-to-right order with intervening newlines.
Any error messages are properly collated to the included file
in which they occurred.

If no query is provided, the inputs are scanned
and output is produced in accordance with `-f` to specify a serialization format
and `-o` to specified an optional output (file or directory).

When invoked using the [db](db.md) sub-command, `super` interacts with
an underlying SuperDB database.

The [dev](dev.md) sub-command provides dev tooling for the advanced users or
developers of SuperDB while the [compile](compile.md) command allows detailed
interactions with various stages of the query compiler.

## Formats

The support input and output formats include the following:

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
> Best performance is typically achieved when operating on data in binary columnar formats
> such as [CSUP](../formats/csup.md),
> [Parquet](https://github.com/apache/parquet-format), or
> [Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format).

## Inputs

When run detached from a database, `super` executes a query over inputs
external to the database including
* file system paths,
* standard input, or
* HTTP, HTTPS, or S3 URLs.

These inputs may be specified from within the query text or via the
command-line arguments (including stdin) to `super`.

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

## Format Detection

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

>[!NOTE]
> Parquet and CSUP require a seekable input and cannot be operated upon
> when read on standard input.

## Errors

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
echo '1 2 0 5' | super -s -c '10/this' -
```
produces
```mdtest-output
10
5
error("divide by zero")
2
```
and
```mdtest-command
echo '1 2 0 5' | super -c '10/this' - | super -s -c 'is_error(this)' -
```
produces just
```mdtest-output
error("divide by zero")
```

## Debugging

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

