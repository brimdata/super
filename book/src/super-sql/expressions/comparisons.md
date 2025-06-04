## Comparisons

Comparison operations (`<`, `<=`, `==`, `=`, `!=`, `>`, `>=`) follow customary syntax
and semantics and result in a truth value of type `bool` or an [error](data-types.md#first-class-errors).
A comparison expression is any valid expression compared to any other
valid expression using a comparison operator.

Values are compared via byte order.  Between values of type `string`, this is
equivalent to [C/POSIX collation](https://www.postgresql.org/docs/current/collation.html#COLLATION-MANAGING-STANDARD)
as found in other SQL databases such as Postgres.

When the operands are coercible to like types, the result is the truth value
of the comparison.  Otherwise, the result is `false`.  To compare values of
different types, consider the [`compare` function](functions/compare.md).

If either operand to a comparison
is `error("missing")`, then the result is `error("missing")`.

For example,
```mdtest-spq
# spq
values 1 > 2, 1 < 2, "b" > "a", 1 > "a", 1 > x
# input
null
# expected output
false
true
true
false
error("missing")
```

TODO: IS NULL, IS NOT NULL
