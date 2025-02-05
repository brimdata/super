---
weight: 9
title: Conventions
heading: Type Conventions
---

[Function](functions/_index.md) arguments and [operator](operators/_index.md) input values are all dynamically typed,
yet certain functions expect certain specific [data types](data-types.md)
or classes of data types. To this end, the function and operator prototypes
in the Zed documentation include several type classes as follows:
* _any_ - any Zed data type
* _float_ - any floating point Zed type
* _int_ - any signed or unsigned Zed integer type
* _number_ - either float or int
* _record_ - any [record](../formats/jsup.md#251-record-type) type

Note that there is no "any" type in Zed as all super-structured data is
comprehensively typed; "any" here simply refers to a value that is allowed
to take on any Zed type.
