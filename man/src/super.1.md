% super(1)
%
% April 2026

# NAME

super - process data with SuperSQL queries

# SYNOPSIS

**super** [*options*] *command*

**super** [*options*] [**-c** *query*] [*file* ...]

# DESCRIPTION

**super** is the command-line tool for interacting with and managing SuperDB. It is organized as a hierarchy of sub-commands but can also be invoked directly to run SuperSQL queries detached from any database storage layer.

When invoked without a sub-command, **super** executes the SuperDB query engine against one or more input sources. Inputs may be specified as command-line paths, referenced within the query itself (e.g., via a **from** operator or SQL FROM clause), or read from standard input when `-` is given as a path. HTTP, HTTPS, and S3 URLs are also accepted as input paths.

If no query is provided, inputs are scanned and output is produced according to the selected format. If no input is provided, the query receives a single `null` value, enabling standalone computation.

Format detection is automatic for files with recognized extensions (`.json`, `.parquet`, `.sup`, etc.) and for standard input. The `line` format cannot be auto-detected and requires explicit `-i line`. Parquet and CSUP require seekable input and cannot be read from standard input.

When writing to a terminal, the default output format is SUP. Otherwise, the default is BSUP. These defaults may be overridden with `-f`, `-s`, or `-S`.

Runtime errors from queries do not halt execution. Instead, they produce first-class error values in the output stream, interleaved with valid results. These can be identified using the **is_error** function. Fatal errors such as missing files cause immediate exit.

Use `-C` to compile and display a query in canonical form without executing it, which is useful for understanding how SuperSQL shortcuts are expanded.

# OPTIONS

## Global Options

**-h**
:   Display help.

**-help**
:   Display help.

**-version**
:   Print version and exit.

**-hidden**
:   Show hidden options.

## Query Options

**-c** *query*
:   SuperSQL query text to execute. May be used multiple times; query fragments are concatenated in order with intervening newlines.

**-I** *file*
:   Source file containing query text. May be used multiple times.

**-e**
:   Stop upon input errors (default "true").

**-stats**
:   Display search stats on stderr (default "false").

**-aggmem** *size*
:   Maximum memory used per aggregate function value in MiB, MB, etc (default "auto(1GiB)").

**-sortmem** *size*
:   Maximum memory used by **sort** in MiB, MB, etc (default "auto(1GiB)").

**-fusemem** *size*
:   Maximum memory used by **fuse** in MiB, MB, etc (default "auto(1GiB)").

**-C**
:   Display parsed AST in a textual format without executing the query (default "false").

**-sam**
:   Execute query in sequential runtime (default "false").

**-vam**
:   Execute query in vector runtime (default "false").

**-dynamic**
:   Disable static type checking of inputs (default "false").

**-samplesize** *n*
:   Values to read per input file to determine type; less than 1 reads all (default "1000").

## Input Options

**-i** *format*
:   Format of input data: `auto`, `arrows`, `bsup`, `csup`, `csv`, `json`, `jsup`, `line`, `parquet`, `sup`, `tsv`, `zeek` (default "auto").

**-csv.delim** *char*
:   CSV field delimiter (default ",").

**-bsup.readmax** *size*
:   Maximum Super Binary read buffer size in MiB, MB, etc (default "auto(1GiB)").

**-bsup.readsize** *size*
:   Target Super Binary read buffer size in MiB, MB, etc (default "auto(512KiB)").

**-bsup.threads** *n*
:   Number of Super Binary read threads; 0 means GOMAXPROCS (default "0").

**-bsup.validate**
:   Validate format when reading Super Binary (default "false").

## Output Options

**-f** *format*
:   Format for output data: `arrows`, `bsup`, `csup`, `csv`, `db`, `json`, `jsup`, `line`, `parquet`, `sup`, `table`, `tsv`, `zeek` (default "bsup").

**-o** *file*
:   Write data to output file.

**-s**
:   Shortcut for `-f sup -pretty=0`, i.e., line-oriented SUP (default "false").

**-S**
:   Shortcut for `-f sup -pretty 2`, i.e., formatted SUP (default "false").

**-j**
:   Shortcut for `-f json -pretty=0`, i.e., line-oriented JSON (default "false").

**-J**
:   Shortcut for `-f json -pretty 2`, i.e., formatted JSON (default "false").

**-pretty** *n*
:   Tab size for pretty-printing JSON and Super JSON output; 0 for newline-delimited output (default "2").

**-color**
:   Enable or disable color formatting for `-S` and db text output (default "true").

**-B**
:   Allow Super Binary to be sent to a terminal output (default "false").

**-bsup.compress**
:   Compress Super Binary frames (default "true").

**-bsup.framethresh** *bytes*
:   Minimum Super Binary frame size in uncompressed bytes (default "524288").

**-noheader**
:   Omit header for CSV and TSV output (default "false").

**-split** *dir*
:   Split output into one file per data type in the specified directory.

**-splitsize** *size*
:   If greater than 0 and `-split` is set, split into files at least this large rather than by data type (default "0B").

**-persist** *regexp*
:   Regular expression to persist type definitions across the stream.

**-unbuffered**
:   Disable output buffering (default "false").

# SUB-COMMANDS

**compile**
:   Compile a SuperSQL query for inspection and debugging; see https://superdb.org/command/compile.html.

**db**
:   Run database commands; see https://superdb.org/command/db.html.

**dev**
:   Run specified development tool; see https://superdb.org/command/dev.html.

# OUTPUT

Output originates in super-structured form and is serialized to the format specified by `-f`. Supported output formats include `arrows`, `bsup`, `csup`, `csv`, `db`, `json`, `jsup`, `line`, `parquet`, `sup`, `table`, `tsv`, and `zeek`.

When writing to a terminal, the default format is SUP. Otherwise, BSUP is the default. The `db` format pretty-prints lake metadata and is the default for `super db` sub-command output.

Schema-rigid formats such as Arrow and Parquet require all output values to conform to a single schema. Heterogeneous super-structured data must be homogenized before writing to these formats, either by using the **fuse** or **blend** operators, or by using `-split` to write one file per distinct type. The `-split` option names output files using the `-o` value as a prefix with a `-<n>.<ext>` suffix.

SUP and JSON output may be pretty-printed with `-pretty`, `-S`, or `-J`. Colorization is enabled by default when writing to a terminal and can be disabled with `-color false`.

The `line` format writes one value per line. String values are printed as-is; non-string values are formatted as SUP. When used as input with `-i line`, each line is read as a string value.

# ERRORS

Fatal errors (e.g., file not found, filesystem full) cause **super** to exit immediately.

Runtime query errors do not halt execution. They appear as first-class error values in the output stream, interleaved with valid results. Use the **is_error** function to filter or inspect them. Errors may be wrapped to provide stack-trace-like debugging output alongside result data.

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

Embed a pipe query search in SQL FROM clause:

```
super -c "
SELECT union(type) as kinds, network_of(srcip) as net
FROM ( from logs.json | ? example.com AND urgent )
WHERE message_length > 100
GROUP BY net
"
```

Or write this as a pure pipe query using SuperSQL shortcuts:

```
super -c "
from logs.json
| ? example.com AND urgent
| message_length > 100
| kinds:=union(type), net:=network_of(srcip) by net
"
```

# SEE ALSO

SuperDB documentation: https://superdb.org

`super` command reference: https://superdb.org/command/super.html

SuperSQL language reference: https://superdb.org/super-sql/intro.html
