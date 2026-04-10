% super(1)
%
% April 2026

# NAME

super — process data with SuperSQL queries

# SYNOPSIS

**super** [*options*] *command*

**super** [*options*] [**-c** *query*] [*file* ...]

# DESCRIPTION

**super** executes SuperSQL queries against data in a variety of formats. It operates either as a standalone query engine (detached from any database) or as the entry point to a hierarchy of sub-commands for managing SuperDB databases, inspecting query compilation, and accessing developer tooling.

When invoked without a sub-command, **super** runs the SuperDB query engine against the specified input files or standard input. Input may also be specified within the query itself using a **from** operator or a SQL `FROM` clause. If no query is given, inputs are scanned and serialized to the output format unchanged, providing a convenient format-conversion tool.

If no input is specified, the query receives a single `null` value, analogous to SQL's default input of a single empty row, enabling use as a calculator or for standalone expressions.

Input paths may be local file paths, `-` for standard input, or HTTP, HTTPS, or S3 URLs. When multiple input files are specified they are processed in order as if concatenated in a single **from** clause.

## Format Detection

**super** automatically infers input formats from file extensions (`.json`, `.parquet`, `.sup`, etc.) or by reading and parsing a sample of the data. Use `-i` to override format detection; when `-i` is specified all input files must share the same format. The `line` format cannot be auto-detected and always requires `-i line` or an explicit `(format line)` option in a **from** operator.

Parquet and CSUP require seekable input and cannot be read from standard input.

## Output

Output is written to standard output by default, or to the file or directory specified by `-o`. When writing to a terminal, the default output format is SUP; otherwise it is BSUP. These defaults may be overridden with `-f`, `-s`, or `-S`.

For schema-rigid output formats (Arrow, Parquet), all values must conform to a single schema. Use the **fuse** or **blend** operators to unify heterogeneous types, or use `-split` to write one file per distinct type.

## Errors

Fatal errors (e.g., file not found, filesystem full) cause **super** to exit immediately. Runtime errors produced by the query itself do not halt execution; instead they appear as first-class error values interleaved with normal output. These can be detected and filtered using the **is_error** function.

## Debugging

Run **super** with `-C` to compile and display a query in canonical form without executing it. This is useful for understanding how abbreviated SuperSQL syntax expands into full pipe-query form. The **debug** operator may be inserted anywhere in a query pipeline to trace intermediate values to standard error.

# OPTIONS

## Global Options

`-h`
:   display help

`-help`
:   display help

`-version`
:   print version and exit

`-hidden`
:   show hidden options (default `false`)

## Query Options

`-c` *query*
:   SuperSQL query text to execute; may be specified multiple times; multiple values are concatenated with intervening newlines

`-I` *file*
:   source file containing query text; may be specified multiple times; concatenated in left-to-right order with `-c` text

`-e`
:   stop upon input errors (default `true`)

`-aggmem` *size*
:   maximum memory used per aggregate function value in MiB, MB, etc. (default `auto(1GiB)`)

`-fusemem` *size*
:   maximum memory used by **fuse** in MiB, MB, etc. (default `auto(1GiB)`)

`-sortmem` *size*
:   maximum memory used by **sort** in MiB, MB, etc. (default `auto(1GiB)`)

`-stats`
:   display search stats on stderr (default `false`)

`-C`
:   display parsed AST in textual form without executing (default `false`)

`-dynamic`
:   disable static type checking of inputs (default `false`)

`-sam`
:   execute query in sequential runtime (default `false`)

`-vam`
:   execute query in vector runtime (default `false`)

`-samplesize` *n*
:   values to read per input file to determine type; `<1` reads all (default `1000`)

## Input Options

`-i` *format*
:   format of input data; one of `auto`, `arrows`, `bsup`, `csup`, `csv`, `json`, `jsup`, `line`, `parquet`, `sup`, `tsv`, `zeek` (default `auto`)

`-e`
:   stop upon input errors (default `true`)

`-csv.delim` *char*
:   CSV field delimiter (default `,`)

`-bsup.readmax` *size*
:   maximum Super Binary read buffer size in MiB, MB, etc. (default `auto(1GiB)`)

`-bsup.readsize` *size*
:   target Super Binary read buffer size in MiB, MB, etc. (default `auto(512KiB)`)

`-bsup.threads` *n*
:   number of Super Binary read threads; `0` uses GOMAXPROCS (default `0`)

`-bsup.validate`
:   validate format when reading Super Binary (default `false`)

## Output Options

`-f` *format*
:   format for output data; one of `arrows`, `bsup`, `csup`, `csv`, `db`, `json`, `jsup`, `line`, `parquet`, `sup`, `table`, `tsv`, `zeek` (default `bsup`)

`-o` *file*
:   write data to output file

`-s`
:   shortcut for `-f sup -pretty=0`, i.e., line-oriented SUP (default `false`)

`-S`
:   shortcut for `-f sup -pretty 2`, i.e., multi-line SUP (default `false`)

`-j`
:   shortcut for `-f json -pretty=0`, i.e., line-oriented JSON (default `false`)

`-J`
:   shortcut for `-f json -pretty 2`, i.e., formatted JSON (default `false`)

`-pretty` *n*
:   tab size for pretty-printing JSON and Super JSON output; `0` for newline-delimited output (default `2`)

`-color`
:   enable/disable color formatting for `-S` and `db` text output (default `true`)

`-B`
:   allow Super Binary to be sent to a terminal output (default `false`)

`-bsup.compress`
:   compress Super Binary frames (default `true`)

`-bsup.framethresh` *bytes*
:   minimum Super Binary frame size in uncompressed bytes (default `524288`)

`-noheader`
:   omit header for CSV and TSV output (default `false`)

`-persist` *regexp*
:   regular expression to persist type definitions across the stream

`-split` *dir*
:   split output into one file per data type in the specified directory

`-splitsize` *size*
:   if `>0` and `-split` is set, split into files at least this large rather than by data type (default `0B`)

`-unbuffered`
:   disable output buffering (default `false`)

# SUB-COMMANDS

`compile`
:   compile a SuperSQL query for inspection and debugging — see <https://superdb.org/command/compile.html>

`db`
:   run database commands — see <https://superdb.org/command/db.html>

`dev`
:   run specified development tool — see <https://superdb.org/command/dev.html>

# OUTPUT

Output originates in super-structured form and is serialized to the format specified by `-f`. The supported output formats are:

`arrows`
:   Arrow IPC Stream Format

`bsup`
:   Super Binary (BSUP); default when writing to a non-terminal

`csup`
:   Columnar Super Binary (CSUP)

`csv`
:   Comma-Separated Values (RFC 4180)

`db`
:   SuperDB lake metadata pretty-print format; default for `super db` metadata commands

`json`
:   JSON (RFC 8259)

`jsup`
:   Super over JSON (JSUP)

`line`
:   one text value per line; string values printed as-is, non-strings formatted as SUP

`parquet`
:   Apache Parquet

`sup`
:   SUP text format; default when writing to a terminal

`table`
:   aligned text table

`tsv`
:   Tab-Separated Values

`zeek`
:   Zeek log format

Best performance is typically achieved with binary columnar formats: `csup`, `parquet`, or `arrows`.

# ERRORS

Fatal I/O errors cause **super** to exit immediately with a non-zero status. Query runtime errors are emitted as first-class error values in the output stream and do not halt execution. Use **is_error** to detect them and **has_error** to test whether any error is present in a value.

# EXAMPLES

Query a CSV, JSON, or Parquet file using SuperSQL:

```
super -c "SELECT * FROM file.[csv|csv.gz|json|json.gz|parquet]"
```

Run a SuperSQL query sourced from an input file:

```
super -I path/to/query.sql
```

Pretty-print a sample value as super-structured data:

```
super -S -c "limit 1" file.[csv|csv.gz|json|json.gz|parquet]
```

Compute a histogram of the "data shapes" in a JSON file:

```
super -c "count() by typeof(this)" file.json
```

Display a sample value of each "shape" of JSON data:

```
super -c "any(this) by typeof(this) | values any" file.json
```

Search Parquet files easily and efficiently without schema handcuffs:

```
super *.parquet > all.bsup
super -c "? search keywords | other pipe processing" all.bsup
```

Read a CSV from stdin, process with a query, and write to stdout:

```
cat input.csv | super -f csv -c <query> -
```

Fuse JSON data into a unified schema and output as Parquet:

```
super -f parquet -o out.parquet -c fuse file.json
```

Run as a calculator:

```
super -c "1.+(1/2.)+(1/3.)+(1/4.)"
```

Search all values in a database pool called logs for keyword "alert" and level >= 2:

```
super db -c "from logs | ? alert level >= 2"
```

Traverse nested data with recursive functions and re-entrant subqueries:

```
super -c '
fn walk(node, visit):
  case kind(node)
  when "array" then
    [unnest node | walk(this, visit)]
  when "record" then
    unflatten([unnest flatten(node) | {key,value:walk(value, visit)}])
  when "union" then
    walk(under(node), visit)
  else visit(node)
  end
fn addOne(node): case typeof(node) when <int64> then node+1 else node end
values 1, [1,2,3], [{x:[1,"foo"]},{y:2}]
| values walk(this, &addOne)
'
```

Handle and wrap errors in a SuperSQL pipeline:

```
... | super -c "
switch is_error(this) (
    case true ( values error({message:\"error into stage N\", on:this}) )
    default (
        <non-error processing here>
        ...
    )
)
"
| ...
```

Embed a pipe query search in a SQL FROM clause:

```
super -c "
SELECT union(type) as kinds, network_of(srcip) as net
FROM ( from logs.json | ? example.com AND urgent )
WHERE message_length > 100
GROUP BY net
"
```

Equivalently, as a pure pipe query using SuperSQL shortcuts:

```
super -c "
from logs.json
| ? example.com AND urgent
| message_length > 100
| kinds:=union(type), net:=network_of(srcip) by net
"
```

# SEE ALSO

SuperDB documentation: <https://superdb.org>

**super** command reference: <https://superdb.org/command/super.html>

SuperSQL language reference: <https://superdb.org/super-sql/intro.html>
