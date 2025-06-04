## Data Types

TODO: update and break this into smaller chunks

The SuperSQL language includes most data types of a typical programming language
as defined in the [super data model](../formats/data-model.md).

The syntax of individual literal values generally follows
the [Super (SUP) syntax](../formats/sup.md) with the exception that
[type decorators](../formats/sup.md#22-type-decorators)
are not included in the language.  Instead, a
[type cast](expressions.md#casts) may be used in any expression for explicit
type conversion.

In particular, the syntax of primitive types follows the
[primitive-value definitions](../formats/sup.md#23-primitive-values) in SUP
as well as the various [complex value definitions](../formats/sup.md#24-complex-values)
like records, arrays, sets, and so forth.  However, complex values are not limited to
constant values like SUP and can be composed from [literal expressions](expressions.md#literals).

XXX

