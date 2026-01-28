# Operators

The components of a SuperSQL [pipeline](../intro.md#pipe-queries)
are called pipe operators.  Each operator is identified by its name
and performs a specific operation on a sequence of values.

Some operators, like
[`aggregate`](aggregate.md) and [`sort`](sort.md),
read all of their input before producing output, though
`aggregate` can produce incremental results when the grouping key is
aligned with the order of the input.

For large queries that process all of their input, time may pass before
seeing any output.

On the other hand, most operators produce incremental output by operating
on values as they are produced.  For example, a long running query that
produces incremental output streams its results as produced, i.e.,
running [`super`](../../command/super.md) to standard output
will display results incrementally.

The [`search`](search.md) and [`where`](where.md)
operators "find" values in their input and drop
the ones that do not match what is being looked for.

The [`values`](values.md) operator emits one or more output values
for each input value based on arbitrary [expressions](../expressions/intro.md),
providing a convenient means to derive arbitrary output values as a function
of each input value.

The [`fork`](fork.md) operator copies its input to parallel
branches of a pipeline, while the [`switch` operator](switch.md)
routes each input value to only one corresponding branch
(or drops the value) based on the switch clauses.

While the output order of parallel branches is [undefined](../intro.md#data-order),
order may be reestablished by applying a [`sort`](sort.md) at the merge point of
the `switch` or `fork`.

## Field Assignment

Several pipe operators manipulate records by modifying fields
or by creating new records from component expressions.

For example,

* the [`put`](put.md) operator adds or modifies fields,
* the [`cut`](cut.md) operator extracts a subset of fields, and
* the [`aggregate`](aggregate.md) operator forms new records from
[aggregate functions](../aggregates/intro.md) and grouping expressions.

In all of these cases, the SuperSQL language uses the syntax `:=` to denote
_field assignment_ and has the form:
```
<field> := <expr>
```

For example,
```
put x:=y+1
```
or
```
aggregate salary:=sum(income) by state:=lower(state)
```
This style of "assignment" to a record value is distinguished from the `=`
symbol, which denotes Boolean equality.

The field name and `:=` symbol may also be omitted and replaced with just the expression,
as in
```
aggregate count() by upper(key)
```
or
```
put lower(s), a.b.c, x+1
```
In this case, the field name is derived using the same
[rules](../types/record.md#derived-field-names) that determine the field
name of an unnamed record field.

In the two examples above, the derived names are filled in as follows:
```
aggregate count:=count() by upper:=upper(key)
put lower:=lower(s), c:=a.b.c, `x+1`:=x+1
```

## Call

In addition to the built-in operators,
[new operators can be declared](../declarations/operators.md)
that take parameters and operate on input just like the built-ins.

A declared operator is called using the `call` keyword:
```
call <id> [<arg> [, <arg> ...]]
```
where `<id>` is the name of the operator and each `<arg>` is an
[expression](../expressions/intro.md) or
[function reference](../expressions/functions.md#function-references).
The number of arguments must match the number
of parameters appearing in the operator declaration.

The `call` keyword is optional when the operator name does not
syntactically conflict with other operator syntax.

## Shortcuts

When interactively composing queries (e.g., within [SuperDB Desktop](https://zui.brimdata.io)),
it is often convenient to use syntactic shortcuts to quickly craft queries for
exploring data interactively as compared to a "coding style" of query writing.

Shortcuts allow certain operator names to be optionally omitted when
they can be inferred from context and are available for:
* [aggregate](aggregate.md),
* [put](put.md),
* [values](values.md), and
* [where](where.md).

For example, the SQL expression
```
SELECT count(),type GROUP BY type
```
is more concisely represented in pipe syntax as
```
aggregate count() by type
```
but even more succinctly expressed as
```
count() by type
```
Here, the syntax of the [aggregate](aggregate.md) operator is unambiguous so
the `aggregate` keyword may be dropped.

Similarly, an [expression](../expressions/intro.md) situated in the position
of a pipe operator implies a [values](values.md) shortcut, e.g.,
```
{a:x+1,b:y-1}
```
is shorthand for
```
values {a:x+1,b:y-1}
```

>[!NOTE]
> The values shortcut means SuperSQL provides a calculator experience, e.g.,
> the command `super -c '1+1'` emits the value `2`.

When the expression is Boolean-valued, however, the shortcut is [where](where.md)
instead of [values](values.md) providing a convenient means to filter values.
For example
```
x >= 1
```
is shorthand for
```
where x >= 1
```

Finally the [put](put.md) operator can be used as a shortcut where a list
of [field assignments](#field-assignment) may omit the `put` keyword.

For example, the operation
```
put a:=x+1,b:=y-1
```
can be expressed simply as
```
a:=x+1,b:=y-1
```
To confirm the interpretation of a shortcut, you can always check the compiler's
actions by running `super` with the `-C` flag to print the parsed query
in a "canonical form", e.g.,
```mdtest-command
super -C -c 'x >= 1'
super -C -c 'count() by type'
super -C -c '{a:x+1,b:y-1}'
super -C -c 'a:=x+1,b:=y-1'
```
produces
```mdtest-output
where x>=1
aggregate
    count() by type
values {a:x+1,b:y-1}
put a:=x+1,b:=y-1
```
When composing long-form queries that are shared via SuperDB Desktop or managed in GitHub,
it is best practice to include all operator names in the query text.
