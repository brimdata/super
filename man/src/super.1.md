% super(1)
%
% April 2026

# NAME

super - process data with SuperSQL queries

# SYNOPSIS

**super** [*options*] *command*

**super** [*options*] [**-c** *query*] [*file* ...]

# DESCRIPTION

**super** executes SuperSQL queries against data in a variety of formats. It operates either as a standalone query engine (detached from any database) or as the entry point to a hierarchy of sub-commands for managing SuperDB databases, inspecting query plans, and accessing developer tooling.

When invoked without a sub-command, **super** reads input from one or more *file* arguments, from standard input (when *file* is **-**), or from sources referenced within the query itself (e.g., via a **from** operator or SQL **FROM** clause). If no query is provided, inputs are scanned and serialized to the output format unchanged, making **super** useful as a format converter.

Input paths may be local file paths, HTTP/HTTPS/S3 URLs, or **-** for standard input. When multiple input files are specified they are processed in order as if concatenated by a single **from** clause.

If no input is specified, the query receives a single **null** value, analogous to SQL's default input of a single empty row. This allows **super** to be used as a calculator or to generate standalone results.

## Format Detection

**super** automatically infers input formats from file extensions (e.g., `.json`, `.parquet`, `.sup`). For standard input or files without a recognized extension, **super** reads and parses a sample of the data to detect the format. Use **-i** to override format inference; when **-i** is specified, all input files must share the same format. The `line` format cannot be auto-detected and always requires **-i line** or an explicit `(format line)` option in a **from** operator.

Parquet and CSUP require seekable input and cannot be read from standard input.

## Output

Output is written to standard output by default, or to the file or directory specified by **-o**. When writing to a terminal, the default output format is SUP; otherwise it is BSUP. These defaults may be overridden with **-f**, **-s**, or **-S**.

For schema-rigid output formats (Arrow, Parquet), all values in the output must conform to a single schema. Use the **fuse** or **blend** operator to unify heterogeneous types, or use **-split** to write one file per distinct type.

## Error Handling

Fatal errors (e.g., file not found, filesystem full) cause **super** to exit immediately. Runtime errors produced by the query itself do not halt execution; instead they appear as first-class error values interleaved with normal output. These can be detected and filtered using the **is_error** function.

## Debugging

Use **-C** to compile and display a query in canonical form without executing it. This is useful for understanding how shorthand syntax is expanded, for example how an implied **where** or **put** operator is parsed. The **debug** operator can be inserted anywhere in a pipeline to trace intermediate values to standard error.

# OPTIONS

## Global Options

**-h**, **-help**
:   Display help.

**-version**
:   Print version and exit.

**-hidden**
:   Show hidden options.

## Query Options

**-c** *query*
:   SuperSQL query text to execute. May be specified multiple times; query fragments are concatenated left-to-right with intervening newlines.

**-I** *file*
:   Source file containing query text. May be specified multiple times; concatenated in order with any **-c** fragments.

**-e**
:   Stop upon input errors (default `true`).

**-aggmem** *size*
:   Maximum memory used per aggregate function value in MiB, MB, etc. (default `auto(1GiB)`).

**-sortmem** *size*
:   Maximum memory used by **sort** in MiB, MB, etc. (default `auto(1GiB)`).

**-fusemem** *size*
:   Maximum memory used by **fuse** in MiB, MB, etc. (default `auto(1GiB)`).

**-stats**
:   Display search stats on stderr (default `false`).

**-C**
:   Display parsed AST in textual form without executing the query (default `false`).

**-sam**
:   Execute query in sequential runtime (default `false`).

**-vam**
:   Execute query in vector runtime (default `false`).

**-dynamic**
:   Disable static type checking of inputs (default `false`).

**-samplesize** *n*
:   Values to read per input file to determine type; values less than 1 read all (default `1000`).

## Input Options

**-i** *format*
:   Format of input data. One of: `auto`, `arrows`, `bsup`, `csup`, `csv`, `json`, `jsup`, `line`, `parquet`, `sup`, `tsv`, `zeek` (default `auto`).

**-e**
:   Stop upon input errors (default `true`).

**-csv.delim** *char*
:   CSV field delimiter (default `,`).

**-bsup.readmax** *size*
:   Maximum Super Binary read buffer size in MiB, MB, etc. (default `auto(1GiB)`).

**-bsup.readsize** *size*
:   Target Super Binary read buffer size in MiB, MB, etc. (default `auto(512KiB)`).

**-bsup.threads** *n*
:   Number of Super Binary read threads; 0 means GOMAXPROCS (default `0`).

**-bsup.validate**
:   Validate format when reading Super Binary (default `false`).

## Output Options

**-f** *format*
:   Format for output data. One of: `arrows`, `bsup`, `csup`, `csv`, `db`, `json`, `jsup`, `line`, `parquet`, `sup`, `table`, `tsv`, `zeek` (default `bsup`).

**-o** *file*
:   Write data to output file.

**-s**
:   Shortcut for `-f sup -pretty=0`; line-oriented SUP output.

**-S**
:   Shortcut for `-f sup -pretty 2`; formatted SUP output.

**-j**
:   Shortcut for `-f json -pretty=0`; line-oriented JSON output.

**-J**
:   Shortcut for `-f json -pretty 2`; formatted JSON output.

**-pretty** *n*
:   Tab size for pretty-printing JSON and SUP output; 0 for newline-delimited output (default `2`).

**-color** *bool*
:   Enable or disable color formatting for **-S** and `db` text output (default `true`).

**-f** *format*
:   Format for output data (see above).

**-noheader**
:   Omit header row for CSV and TSV output (default `false`).

**-split** *dir*
:   Split output into one file per data type in the specified directory.

**-splitsize** *size*
:   If greater than 0 and **-split** is set, split into files of at least this size rather than by data type (default `0B`).

**-persist** *regexp*
:   Regular expression to persist type definitions across the stream.

**-B**
:   Allow Super Binary to be sent to a terminal output (default `false`).

**-bsup.compress**
:   Compress Super Binary frames (default `true`).

**-bsup.framethresh** *bytes*
:   Minimum Super Binary frame size in uncompressed bytes (default `524288`).

**-unbuffered**
:   Disable output buffering (default `false`).

# SUB-COMMANDS

**compile**
:   Compile a SuperSQL query for inspection and debugging. See https://superdb.org/command/compile.html

**db**
:   Run database commands. See https://superdb.org/command/db.html

**dev**
:   Run specified development tool. See https://superdb.org/command/dev.html

# OUTPUT

Output is always produced in super-structured form internally and serialized to the format specified by **-f** (or its shortcuts). Super-structured formats (BSUP, CSUP, SUP, JSUP) preserve the full type richness of the data model and are pipeline-friendly: output from one **super** invocation can be piped directly into another without schema registration.

Arrow and Parquet are schema-rigid: all output values must share a single schema. Use **fuse** or **blend** to unify types before writing, or use **-split** to produce one file per type. The **blend** operator merges record types using type fusion, inserting nulls where fields are absent.

The `line` output format writes one value per line. String values are printed as-is with escape sequences rendered as their native characters. Non-string values are formatted as SUP.

The `db` format pretty-prints lake metadata and is the default for `super db` sub-command output.

Runtime errors appear as first-class error values in the output stream, interleaved with valid results. Use **is_error** to filter or route them.

# ERRORS

Fatal I/O errors (missing files, full filesystem) terminate **super** immediately with a non-zero exit status.

Query runtime errors (e.g., divide by zero, type mismatches) do not halt execution. They are emitted as typed error values in the output stream. Use **is_error** to detect them and **quiet** to suppress them.

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

Write the same query as a pure pipe query using SuperSQL shortcuts:

```
super -c "
from logs.json
| ? example.com AND urgent
| message_length > 100
| kinds:=union(type), net:=network_of(srcip) by net
"
```

# SEE ALSO

SuperDB home: https://superdb.org

Command reference: https://superdb.org/commands/super.html

SuperSQL reference: https://superdb.org/super-sql/intro.html
