## Data Types

SuperSQL has a comprehensive types system that adheres to the
[super-structured data model](../../formats/model.md)
comprising 
[primitive types](#primitive-types),
[complex types](#complex-types), 
[sum types](union.md),
[named types](named.md),
the [null type](null.md),
and _first class_
[errors](error.md) and [types](type.md).

The syntax of individual literal values follows
the [SUP format](../../formats/sup.md) in that any legal
SUP value is also valid SuperSQL literal.
In particular, the type decorators in SUP utilize a double colon (`::`)
syntax that is compatible with the SuperSQL
[`cast`](../expressions.md#casts) operator.

### Primitive Types

* [Number Types](numbers.md)
* [String](string.md)
* [Bytes](bytes.md)
* [Network Types](network.md)
* [Time Types](time.md)
* [Type Type](type.md)
* [Null](null.md)

### Complex Types

* [Records](record.md)
* [Arrays](array.md)
* [Sets](set.md)
* [Maps](map.md)
* [Union](union.md)
* [Enums](enum.md)
* [Errors](enum.md)
