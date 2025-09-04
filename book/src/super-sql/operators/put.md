### Operator

&emsp; **put** &mdash; add or modify fields of records

### Synopsis
```
[put] <assignment> [, <assignment> ...]
```
where `<assignment>` is a [field assignment](intro.md#field-assignment)
having the form:
```
[ <field> := ] <expr>
```
### Description

The `put` operator modifies its input with
one or more [field assignments](intro.md#field-assignment).

Each expression `<expr>` is evaluated based on the input value
and the result is either assigned to a new field of the input record if it does not
exist, or the existing field is modified in its original location with the result.

New fields are appended in left-to-right order to the right of existing record fields
while modified fields are mutated in place.

If multiple fields are written in a single `put`, all the new field values are
computed first and then they are all written simultaneously.  As a result,
a computed value cannot be referenced in another expression.  If you need
to re-use a computed result, this can be done by chaining multiple `put` operators.

The `put` keyword is optional since it can be used as a [shortcut](../shortcuts.md).
When used as a shortcut, the `<field>:=` portion of `<assignment>` is not optional.

Each left-hand side `<field>` term must be a field reference expressed as
a dotted path or sequence of constant index operations on `this`, e.g., `a.b`.

Each right-hand side `<expr>` can be any expression.

For any input value that is not a record, a structured error is emitted
having the form:
```
error({message:"put: not a record",on:<value>})
```
where `<value>` is the offending input value.

Note that when the field references are all top level,
`put` is a special case of [`values`](values.md)
with a [record expression](../expressions.md#record-expressions)
using a spread operator of the form:
```
values {...this, <field>:<expr> [, <field>:<expr>...]}
```

### Examples

---

_A simple put_
```mdtest-spq
# spq
put c:=3
# input
{a:1,b:2}
# expected output
{a:1,b:2,c:3}
```

---

_The `put` keyword may be omitted_
```mdtest-spq
# spq
c:=3
# input
{a:1,b:2}
# expected output
{a:1,b:2,c:3}
```

---

_A `put` operation can also be done with a record spread_
```mdtest-spq
# spq
values {...this, c:3}
# input
{a:1,b:2}
# expected output
{a:1,b:2,c:3}
```

---

_Missing fields show up as missing errors_
```mdtest-spq
# spq
put d:=e
# input
{a:1,b:2,c:3}
# expected output
{a:1,b:2,c:3,d:error("missing")}
```

---

_Non-record input values generate errors_
```mdtest-spq {data-layout="stacked"}
# spq
b:=2
# input
{a:1}
1
# expected output
{a:1,b:2}
error({message:"put: not a record",on:1})
```
