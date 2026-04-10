% super(1)
%
% April 2026

# NAME

super — process data with SuperSQL queries

# SYNOPSIS

**super** [*options*] *command*

**super** [*options*] [**-c** *query*] [*file* ...]

# DESCRIPTION

**super** executes SuperSQL queries against data from files, URLs, or standard input, and writes results to standard output or a file. It operates either as a standalone query engine (detached from any database) or as the entry point to a hierarchy of sub-commands for database management, query compilation, and developer tooling.

When invoked without a sub-command, **super** runs the query engine against the specified inputs. Inputs may be specified as command-line path arguments or referenced within the query itself using a **from** operator or SQL FROM clause. When the path argument is **-**, input is read from standard input. HTTP, HTTPS, and S3 URLs are also accepted as paths.

If no query is provided, inputs are scanned and emitted in the output format specified by **-f**. If no input is provided, the query receives a single **null** value, enabling calculator-style use.

Input formats are detected automatically by file extension or by sampling the data. The **line** format cannot be auto-detected and must be requested explicitly with **-i line**. Parquet and CSUP require seekable input and cannot be read from standard input.

When writing to a terminal, the default output format is SUP. Otherwise, it is BSUP. These defaults are overridden by **-f**, **-s**, or **-S**.

Multiple **-c** and **-I** options may be combined; their text is concatenated in left-to-right order with intervening newlines.

## Errors

Fatal errors (e.g., file not found, filesystem full) cause **super** to exit immediately. Runtime errors produced by the query itself do not halt execution; instead they appear as first-class error values interleaved with valid output in the result stream. Use the **is_error** function to identify and filter these values.

## Debugging

Run **super -C -c** *query* to display the parsed, canonical form of a query without executing it. This is useful for understanding how shorthand syntax is expanded — for example, bare expressions become implied **where** or **put** operators. The **debug** operator can be inserted anywhere in a pipeline to trace intermediate values to standard error.

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
:   SuperSQL query text to execute. May be used multiple times; text is concatenated in order.

**-I** *file*
:   Source file containing query text. May be used multiple times.

**-e**
:   Stop upon input errors (default `true`).

**-stats**
:   Display search stats on stderr (default `false`).

**-aggmem** *size*
:   Maximum memory used per aggregate function value in MiB, MB, etc (default `auto(1GiB)`).

**-sortmem** *size*
:   Maximum memory used by **sort** in MiB, MB, etc (default `auto(1GiB)`).

**-fusemem** *size*
:   Maximum memory used by **fuse** in MiB, MB, etc (default `auto(1GiB)`).

**-C**
:   Display parsed AST in textual (canonical) form without executing (default `false`).

**-dynamic**
:   Disable static type checking of inputs (default `false`).

**-sam**
:   Execute query in sequential runtime (default `false`).

**-vam**
:   Execute query in vector runtime (default `false`).

## Input Options

**-i** *format*
:   Format of input data. One of: `auto`, `arrows`, `bsup`, `csup`, `csv`, `json`, `jsup`, `line`, `parquet`, `sup`, `tsv`, `zeek` (default `auto`).

**-csv.delim** *char*
:   CSV field delimiter (default `,`).

**-bsup.readmax** *size*
:   Maximum Super Binary read buffer size in MiB, MB, etc (default `auto(1GiB)`).

**-bsup.readsize** *size*
:   Target Super Binary read buffer size in MiB, MB, etc (default `auto(512KiB)`).

**-bsup.threads** *n*
:   Number of Super Binary read threads; 0 means GOMAXPROCS (default `0`).

**-bsup.validate**
:   Validate format when reading Super Binary (default `false`).

**-samplesize** *n*
:   Values to read per input file to determine type; values less than 1 read all (default `1000`).

## Output Options

**-f** *format*
:   Format for output data. One of: `arrows`, `bsup`, `csup`, `csv`, `db`, `json`, `jsup`, `line`, `parquet`, `sup`, `table`, `tsv`, `zeek` (default `bsup`).

**-o** *file*
:   Write data to output file.

**-s**
:   Shortcut for `-f sup -pretty 0` (line-oriented SUP).

**-S**
:   Shortcut for `-f sup -pretty 2` (formatted SUP).

**-j**
:   Shortcut for `-f json -pretty 0` (line-oriented JSON).

**-J**
:   Shortcut for `-f json -pretty 2` (formatted JSON).

**-pretty** *n*
:   Tab size for pretty-printing JSON and SUP output; 0 for newline-delimited output (default `2`).

**-color**
:   Enable/disable color formatting for **-S** and db text output (default `true`).

**-noheader**
:   Omit header for CSV and TSV output (default `false`).

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

Output originates in super-structured form and is serialized to the requested format. Supported output formats are: `arrows`, `bsup`, `csup`, `csv`, `db`, `json`, `jsup`, `line`, `parquet`, `sup`, `table`, `tsv`, `zeek`.

When stdout is a terminal, the default format is SUP; otherwise BSUP. Override with **-f**, **-s**, or **-S**.

Schema-rigid formats (Arrow, Parquet) require all output values to conform to a single schema. To write heterogeneous super-structured data to such formats, either apply the **fuse** or **blend** operator to unify types, or use **-split** to write one file per type. Attempting to write multiple record types to a single Parquet file without fusion produces an error.

The `line` output format emits one value per line. String values are printed as-is with escape sequences rendered as their native characters. Non-string values are formatted as SUP.

The `db` format pretty-prints lake metadata and is the default for many `super db` sub-commands.

# ERRORS

Fatal I/O errors terminate the process immediately with a non-zero exit status. Query runtime errors produce first-class error values in the output stream rather than halting execution. These can be detected with the **is_error** function and inspected or re-wrapped for stack-trace-like diagnostics.

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

Or write this as a pure pipe query:

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

Command reference: https://superdb.org/commands/

SuperSQL reference: https://superdb.org/super-sql/
