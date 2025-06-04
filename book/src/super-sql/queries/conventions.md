## Conventions

TODO: clean this up.  explain "any" in terms of polymorphic operators
on strongly typed data.

[Function](functions/_index.md) arguments and [operator](operators/_index.md) input values are all dynamically typed,
yet certain functions expect certain specific [data types](data-types.md)
or classes of data types. To this end, the function and operator prototypes
in the Zed documentation include several type classes as follows:
* _any_ - any SuperSQL data type
* _float_ - any floating point type
* _int_ - any signed or unsigned integer type
* _number_ - either _float_ or _int_
* _record_ - any [record type](types/record.md)

Note that there is no "any" type in SuperSQL as all super-structured data is
strongly typed; "any" here simply refers to a value that is allowed
to take on any type.

XXX mention syntax blocks `<expr>` refers to expression, `<id>` to identifier, etc

Lexical structure goes here?
