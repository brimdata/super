# Input

The `super` command and the `super db load` commands take input arguments.

A `super db -c` query can refer to HTTP inputs but not file-system paths
or stdin.

The inputs for the `super` command may be specified either
* within the query itself, e.g., using a
[from](../super-sql/operators/from.md) operator or a
SQL [FROM](../super-sql/sql/from.md) clause,
* as command-line arguments indicated by one more `<path>` parameters, or
* from standard input when the `<path>` argument is specified as dash (`-`).

The `super db load` command does not run a query and
takes input only as `<path>` arguments.

A `<path>` argument can be
* standard input or
* file-system path relative to the directory in which `super` runs, or
* HTTP, HTTPS, or S3 URLs.

Command-line `<path>` arguments are treated as if a
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
