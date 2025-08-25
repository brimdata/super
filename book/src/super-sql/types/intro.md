## Data Types

SuperSQL has a comprehensive types system that adheres to the
[super-structured data model](../../formats/model.md)
comprising 
[primitive types](../../formats/model.md#1-primitive-types),
[complex types](../../formats/model.md#2-complex-types),
[sum types](union.md),
[named types](named.md),
the [null type](null.md),
and _first class_
[errors](error.md) and [types](type.md).

The syntax of individual literal values follows
the [SUP format](../../formats/sup.md) in that any legal
SUP value is also a valid SuperSQL literal.
In particular, the type decorators in SUP utilize a double colon (`::`)
syntax that is compatible with the SuperSQL
[`cast`](../expressions.md#casts) operator.
