% super(1)
%
% April 2026

# NAME

**super** — process data with SuperSQL queries

# SYNOPSIS

**super** [*options*] [**-c** *query*] [**-I** *query-file* ...] [*file* ...]

**super** [*options*] *command* [*args* ...]

# DESCRIPTION

`super` executes SuperSQL queries against files, HTTP/HTTPS/S3 URLs, and
standard input, independent of a database storage layer.
When invoked without a sub-command, the query engine runs detached from any
database.
Input paths may be filesystem paths, URLs, or **-** for standard input.
If no query is given, inputs are scanned and re-serialized in the format
specified by **-f**.
If no input is specified, the query receives a single **null** value.

Input format is detected automatically from file extensions or content.
The **line** format cannot be auto-detected and must be specified with
**-i line**.

# OPTIONS

## Global Options

**-h**, **-help**
:   Display help and exit. (default: false)

**-hidden**
:   Show hidden options. (default: false)

**-version**
:   Print version and exit. (default: false)

## Query Options

**-c** *query*
:   SuperSQL query text. May be specified multiple times; queries are
    concatenated in order.

**-I** *file*
:   Read query text from *file*. May be repeated.

**-C**
:   Compile and display the query as canonical SuperSQL text without
    executing it. Output goes to stdout; **-f** and **-o** have no
    effect. (default: false)

**-dynamic**
:   Disable static type checking of inputs. (default: false)

**-sam**
:   Execute query using the sequential runtime. (default: false)

**-vam**
:   Execute query using the vector runtime. (default: false)

**-stats**
:   Emit search statistics to stderr after execution. (default: false)

**-e**
:   Stop execution on input errors. (default: true)

**-aggmem** *size*
:   Maximum memory per aggregate function value. (default: auto(1GiB))

**-fusemem** *size*
:   Maximum memory for fuse operations. (default: auto(1GiB))

**-sortmem** *size*
:   Maximum memory for sort operations. (default: auto(1GiB))

## Input Options

**-i** *format*
:   Input format. One of: `auto`, `arrows`, `bsup`, `csup`, `csv`,
    `json`, `jsup`, `line`, `parquet`, `sup`, `tsv`, `zeek`.
    (default: auto)

**-csv.delim** *char*
:   CSV field delimiter. (default: `,`)

**-samplesize** *n*
:   Number of values read per input file for type inference.
    Values less than 1 read all. (default: 1000)

**-bsup.readmax** *size*
:   Maximum BSUP read buffer size. (default: auto(1GiB))

**-bsup.readsize** *size*
:   Target BSUP read buffer size. (default: auto(512KiB))

**-bsup.threads** *n*
:   Number of BSUP read threads. 0 uses GOMAXPROCS. (default: 0)

**-bsup.validate**
:   Validate BSUP format when reading. (default: false)

## Output Options

**-f** *format*
:   Output format. One of: `arrows`, `bsup`, `csup`, `csv`, `db`,
    `json`, `jsup`, `line`, `parquet`, `sup`, `table`, `tsv`, `zeek`.
    (default: bsup)

**-o** *path*
:   Write output to *path* (file or directory).

**-s**
:   Line-oriented SUP output, independent of **-f**. (default: false)

**-S**
:   Formatted SUP output, independent of **-f**. (default: false)

**-j**
:   Line-oriented JSON output, independent of **-f**. (default: false)

**-J**
:   Formatted JSON output, independent of **-f**. (default: false)

**-pretty** *n*
:   Indentation width for JSON and SUP output. 0 produces
    newline-delimited output. (default: 2)

**-B**
:   Allow binary (BSUP) output to be sent to a terminal. (default: false)

**-color** *bool*
:   Enable or disable colorized output. (default: true)

**-noheader**
:   Omit header row for CSV and TSV output. (default: false)

**-persist** *regex*
:   Persist type definitions matching *regex* across the output stream.

**-split** *dir*
:   Split output into one file per data type in *dir*.

**-splitsize** *size*
:   When **-split** is set and *size* > 0, split into files of at least
    *size* rather than by data type. (default: 0B)

**-unbuffered**
:   Disable output buffering. (default: false)

**-bsup.compress**
:   Compress BSUP frames. (default: true)

**-bsup.framethresh** *bytes*
:   Minimum uncompressed BSUP frame size before compression.
    (default: 524288)

# SUB-COMMANDS

**compile**
:   Compile a SuperSQL query and emit its AST or runtime DAG for
    inspection. See <https://superdb.org/command/compile.html>.

**db**
:   Manage and query SuperDB databases (early in development).
    See <https://superdb.org/command/db.html>.

**dev**
:   Developer utilities. See <https://superdb.org/command/dev.html>.

# OUTPUT

Output is serialized according to **-f**.
When stdout is a terminal and **-f** is not specified, output defaults
to `sup` (human-readable); otherwise it defaults to `bsup` (binary).

When writing to schema-rigid formats (Arrow, Parquet), all values must
conform to a single schema. Use `fuse` or `blend` in the query to unify
types, or use **-split** to write one file per distinct type.

# ERRORS

Query runtime errors do not halt execution. Instead, they appear as
first-class error values interleaved with normal output in the data
stream. Use the `is_error()` function in a subsequent query to filter
them. Fatal system errors (e.g., file not found, disk full) terminate
execution immediately.

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

Search Parquet files easily and efficiently:

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

Embed a pipe query search in SQL FROM clause:

```
super -c "
SELECT union(type) as kinds, network_of(srcip) as net
FROM ( from logs.json | ? example.com AND urgent )
WHERE message_length > 100
GROUP BY net
"
```

# SEE ALSO

SuperDB documentation: <https://superdb.org>

`super` command reference: <https://superdb.org/command/super.html>

SuperSQL language reference: <https://superdb.org/super-sql/intro.html>
