# Output

Many commands produce output, which always originates in super-structured form,
but can be serialized into a number of [supported formats](formats.md).
The super-structured formats are generally
preferred because they retain the full richness of the super-structured
data model.

Output is written to standard output by default or, if `-o` is specified,
to the indicated file or directory.

When writing to stdout and stdout is a terminal, the default
output format is [SUP](../formats/sup.md).
Otherwise, the default format is [BSUP](../formats/bsup.md).
These defaults may be overridden with `-f`, `-s`, or `-S`.

>[!NOTE]
> While BSUP is currently default, a forthcoming release will change
> CSUP to default after CSUP supports streaming.

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
> command line but fails in CI).  However, this avoids problematic performance where a
> data pipeline deployed to production accidentally uses SUP instead of CSUP.
> Since `super` gracefully handles any input, this would be hard to detect.
> Alternatively, making CSUP always be default would cause much annoyance when
> binary data is written to the terminal.

If no query is specified with `-c`, the inputs are scanned without modification
and output in the specified format
providing a convenient means to convert files from one format to another, e.g.,
```
super -f arrows -o out.arrows file1.json file2.parquet file3.csv
```

## Pretty Printing

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

## Pipeline-friendly Formats

Though they're compressed formats, CSUP and BSUP data are self-describing and
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

## Schema-rigid Outputs

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

## Fused Data

The [fuse](../super-sql/operators/fuse.md) operator uses
[type fusion](../super-sql/type-fusion.md) to merge different record
types into a blended type, e.g.,
```mdtest-command
echo '{x:1}{s:"hello"}' | super -o out.parquet -f parquet -c fuse -
super -s out.parquet
```
which produces
```mdtest-output
{x:1::(int64|null),s:null::(string|null)}
{x:null::(int64|null),s:"hello"::(string|null)}
```
The downside of this approach is that the data must be changed (by inserting nulls)
to conform to a single type.

Also, data fusion can sometimes involve sum types that are not
representable in a format like Parquet.  While a bit cumbersome,
you could write a query that adjusts the output be renaming columns
so that heterogenous data column types are avoided.   This modified
data could then be fused without sum types and output to Parquet.

## Splitting Schemas

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

## Database Metadata

> **TODO: We should get rid of this.  Or document it as an internal format.
> It's not a format that people should rely upon.**

The `db` format is used to pretty-print lake metadata, such as in
[`super db` sub-command](db.md) outputs.  Because it's `super db`'s default output format,
it's rare to request it explicitly via `-f`.  However, since it's possible for
`super db` to generate output in any supported format,
the `db` format is useful to reverse this.

For example, imagine you'd executed a [meta-query](db.md#meta-queries) via
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

## Line Format

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
