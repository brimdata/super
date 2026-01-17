## Logic

The keywords `and`, `or`, `not`, and `!` perform logic on operands of type [bool](../types/bool.md).
The binary operators `and` and `or` operate on Boolean values and result in
an [error](../types/error.md) value if either operand is not a Boolean.  Likewise, `not` (and its
equivalent `!`) operates on its unary operand and results in an error if its
operand is not type `bool`. Unlike many other languages, non-Boolean values are
_not_ automatically converted to Boolean type using "truthiness" heuristics.
