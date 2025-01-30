### Operator

&emsp; **cut** &mdash; extract subsets of record fields into new records

### Synopsis

```
cut <field>[:=<expr>] [, <field>[:=<expr>] ...]
```
### Description

The `cut` operator extracts values from each input record in the
form of one or more [field assignments](../pipeline-model.md#field-assignments),
creating one field for each expression.  Unlike the `put` operator,
which adds or modifies the fields of a record, `cut` retains only the
fields enumerated, much like a SQL projection.

Each `<field>` expression must be a field reference expressed as a dotted path or sequence of
constant index operations on `this`, e.g., `a.b` or `a["b"]`.

Each right-hand side `<expr>` can be any Zed expression and is optional.

When the right-hand side expressions are omitted,
the _cut_ operation resembles the Unix shell command, e.g.,
```
... | cut a,c | ...
```
If an expression results in `error("quiet")`, the corresponding field is omitted
from the output.  This allows you to wrap expressions in a `quiet()` function
to filter out missing errors.

If an input value to cut is not a record, then the cut still operates as defined
resulting in `error("missing")` for expressions that reference fields of `this`.

Note that when the field references are all top level,
`cut` is a special case of a yield with a
[record literal](../expressions.md#record-expressions) having the form:
```
yield {<field>:<expr> [, <field>:<expr>...]}
```

### Examples

_A simple Unix-like cut_
```mdtest-spq
# spq
cut a,c
# input
{a:1,b:2,c:3}
# expected output
{a:1,c:3}
```

_Missing fields show up as missing errors_
```mdtest-spq
# spq
cut a,d
# input
{a:1,b:2,c:3}
# expected output
{a:1,d:error("missing")}
```

_The missing fields can be ignored with quiet_
```mdtest-spq
# spq
cut a:=quiet(a),d:=quiet(d)
# input
{a:1,b:2,c:3}
# expected output
{a:1}
```

_Non-record values generate missing errors for fields not present in a non-record `this`_
```mdtest-spq {data-layout="stacked"}
# spq
cut a,b
# input
1
{a:1,b:2,c:3}
# expected output
{a:error("missing"),b:error("missing")}
{a:1,b:2}
```

_Invoke a function while cutting to set a default value for a field_

{{% tip "Tip" %}}

This can be helpful to transform data into a uniform record type, such as if
the output will be exported in formats such as `csv` or `parquet` (see also:
[`fuse`](fuse.md)).

{{% /tip %}}

```mdtest-spq
# spq
cut a,b:=coalesce(b, 0)
# input
{a:1,b:null}
{a:1,b:2}
# expected output
{a:1,b:0}
{a:1,b:2}
```
