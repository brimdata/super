## Functions

An invocation of a built-in function may appear in any
[expression](../expressions/intro.md).
A function takes zero or more positional arguments and always produces
a single output value.  There are no named function parameters.

A [declared function](../declarations/functions.md) whose name conflicts with a
built-in function name overrides the built-in function.

Functions are generally polymorphic and can be called with values of varying type
as their arguments.  When type errors occur, functions will return structured errors
reflecting the error.

>[!NOTE]
> Static type checking of function arguments and return values is not yet implemented
> in SuperSQL but will be supported in a future version.

Throughout the function documentation, expected parameter types and the return type
are indicated with type signatures having the form
```
<name> ( [ <formal> : <type> ] [ , <formal> : <type> ] ) -> <type>
```
where `<name>` is the function name, `<formal>` is an identifier representing
the formal name of a function parameter,
and `<type>` is either the name of an actual [type](../types/intro.md)
or a documentary pseudo-type indicating categories defined as follows:
* _any_ - any SuperSQL data type
* _float_ - any [floating point](../types/numbers.md#floating-point) type
* _int_ - any [signed](../types/numbers.md#signed-integers) or
    [unsigned](../types/numbers.md#unsigned-integers) integer type
* _number_ - any [numeric](../types/numbers.md) type
* _record_ - any [record type](../types/record.md)
