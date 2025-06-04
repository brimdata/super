## Inputs

An input to an expression is analagous to a read-only variable that references
the input data to the operator in which the expression appears.

In pipe operators, there is one and only input referenced with the
indentifier `this`.

XXX

For expressions that appear in pipe operators,
input is referenced using [dataflow scoping](../../intro.md#dataflow-scoping),
where all input is referenced as a single value called `this`.

The type of `this` may be any [type](../../types/intro.md).
When `this` is a [record](../../types/record.md), references
to fields of the record may be referenced by an indentifier that names the
field of the implied `this` value, e.g., `x` means `this.x`.

For expressions that appear in a [SQL operator](../../sql/intro.md),
input is presumed to be in the form of records and is referenced using
[relational scoping](../../intro.md#relational-scoping).
Here, identifiers refer to table aliases and/or column names
and are bound to the available inputs based on SQL semantics.
When the input schema is known and is static, these references are
statically checked and compile-time errors are raised when unknown
tables or columns are referenced.

> _In a future version of SuperSQL, static type analysis will also
> apply to input references in pipe operators using super-structured
> type analysis instead of relational schemas._

When non-record data is referenced in a SQL operator and the input
schema is dynamic and unknown, structured errors will generally arise
and be present in the output data.  These errors can be used to debug
the problem as the offending values will be present in the `on` field
of the structured errors.

XXX actually these will usually be error missing...
