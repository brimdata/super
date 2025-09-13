## Logic

The keywords `and`, `or`, `not`, and `!` perform logic on operands of type `bool`.
The binary operators `and` and `or` operate on Boolean values and result in
an error value if either operand is not a Boolean.  Likewise, `not` (and its
equivalent `!`) operates on its unary operand and results in an error if its
operand is not type `bool`. Unlike many other languages, non-Boolean values are
not automatically converted to Boolean type using "truthiness" heuristics.
