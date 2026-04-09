% super(1)
%
% April 2026

# NAME

**super** — process data with SuperSQL queries

# SYNOPSIS

**super** [*options*] [**-c** *query*] [**-I** *query-file* ...] [*file* ...]

**super** [*options*] *command* [*args* ...]

# DESCRIPTION

`super` runs SuperSQL queries over files, standard input, HTTP endpoints,
S3 paths, and SuperDB databases.
When invoked without a sub-command, the query engine runs detached from any
database storage layer.
Sub-commands provide database management, query compiler inspection, and
developer utilities.

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

# ERRORS

Query runtime errors do not halt execution. Instead, they appear as
first-class error values interleaved with normal output in the data
stream. Use the `is_error()` function in a subsequent query to filter
them. Fatal system errors (e.g., file not found, disk full) terminate
execution immediately.

# EXAMPLES

Query a CSV, JSON, or Parquet file:

```
super -c "SELECT * FROM file.csv"
super -c "SELECT * FROM file.json"
super -c "SELECT * FROM file.parquet"
```

Run a query sourced from a file:

```
super -I path/to/query.sql
```

Pretty-print a sample value as super-structured data:

```
super -S -c "limit 1" file.json
```

Compute a histogram of data shapes in a JSON file:

```
super -c "count() by typeof(this)" file.json
```

Display a sample value of each distinct data shape:

```
super -c "any(this) by typeof(this) | values any" file.json
```

Fuse JSON data into a unified schema and write as Parquet:

```
super -f parquet -o out.parquet -c fuse file.json
```

Combine multiple Parquet files and search with a keyword:

```
super *.parquet > all.bsup
super -c "? search_term | count() by field" all.bsup
```

Read CSV from stdin, process, and write CSV to stdout:

```
cat input.csv | super -f csv -c "SELECT * WHERE value > 10" -
```

Embed a pipe query search within a SQL FROM clause:

```
super -c "
SELECT union(type) AS kinds, network_of(srcip) AS net
FROM ( from logs.json | ? example.com AND urgent )
WHERE message_length > 100
GROUP BY net
"
```

# SEE ALSO

SuperDB documentation: <https://superdb.org>

`super` command reference: <https://superdb.org/command/super.html>

SuperSQL language reference: <https://superdb.org/super-sql/intro.html>
