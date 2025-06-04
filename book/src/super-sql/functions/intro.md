# Functions

An invocation of a built-in functions may appear in any
[expression](../expressions.md).
A function takes zero or more positional arguments and always produce
a single output value.  There are no named function parameters.

User-defined functions whose name conflicts with a built-in function name override
the built-in function.

Functions are generally polymorphic and can be called with values of any type
as their arguments.  When type errors occur, functions will return structured errors
reflecting the error.

> _Static type checking of function arguments and return values is not yet implemented
> in SuperSQL but will be supported in a future version._

Throughout the function documentation, expected parameter types and the return type
are indicated with type signatures having the form
```
<name> ( [ <formal> : <type> ] [ , <formal> : <type> ] ) -> <type>
```
where `<name>` is the function name, `<format>` is a descriptive name of a function paramter,
and `<type>` is either the name of an actual [type](../types/intro.md)
or a documentary pseudo-type indicating categories defined as follows:
* _any_ - any SuperSQL data type
* _float_ - any [floating point](../types/numbers.md#floating-point) type
* _int_ - any [signed](../types/numbers.md#signed-integers) or
    [unsigned](../types/numbers.md#unsigned-integers) integer type
* _number_ - any [numeric](../types/numbers.md) type
* _record_ - any [record type](../types/record.md)
