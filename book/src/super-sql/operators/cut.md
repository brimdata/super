# cut

[✅](../intro.md#data-order)&ensp; extract subsets of record fields into new records

## Synopsis

```
cut <assignment> [, <assignment> ...]
```
where `<assignment>` is a [field assignment](intro.md#field-assignment)
having the form:
```
[ <field> := ] <expr>
```

## Description

The `cut` operator extracts values from each input record in the
form of one or more [field assignments](intro.md#field-assignment),
creating one field for each expression.  Unlike the [`put`](put.md) operator,
which adds or modifies the fields of a record, `cut` retains only the
fields enumerated, much like a SQL [`SELECT`](../sql/select.md) clause.

Each left-hand side `<field>` term must be a field reference expressed as
a dotted path or sequence of constant index operations on `this`, e.g., `a.b`.

Each right-hand side `<expr>` can be any expression and is optional.

When the left-hand side assignments are omitted and the expressions are
simple field references, the _cut_ operation resembles the Unix shell command, e.g.,
```
... | cut a,c | ...
```
If an expression results in `error("quiet")`, the corresponding field is omitted
from the output.  This allows you to wrap expressions in a `quiet()` function
to filter out missing errors.

If an input value to cut is not a record, then cut still operates as defined
resulting in `error("missing")` for expressions that reference fields of `this`.

Note that when the field references are all top level,
`cut` is a special case of
[`values`](values.md) with a
[record expression](../types/record.md) having the form:
```
values {<field>:<expr> [, <field>:<expr>...]}
```

## Examples

---

_A simple Unix-like cut_
```mdtest-spq
# spq
cut a,c
# input
{a:1,b:2,c:3}
# expected output
{a:1,c:3}
```

---

_Missing fields show up as missing errors_
```mdtest-spq
# spq
cut a,d
# input
{a:1,b:2,c:3}
# expected output
{a:1,d:error("missing")}
```

---

_The missing fields can be ignored with quiet_
```mdtest-spq
# spq
cut a:=quiet(a),d:=quiet(d)
# input
{a:1,b:2,c:3}
# expected output
{a:1,d:error("quiet")}
```

---

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

---

_Invoke a function while cutting to set a default value for a field_

>[!TIP]
> This can be helpful to transform data into a uniform record type, such as if
> the output will be exported in formats such as `csv` or `parquet` (see also:
> [`fuse`](fuse.md)).

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

---

_Field names are inferred when the left-hand side of assignment is omitted_

```mdtest-spq
# spq
cut a,coalesce(b, 0)
# input
{a:1,b:null}
{a:1,b:2}
# expected output
{a:1,coalesce:0}
{a:1,coalesce:2}
```
