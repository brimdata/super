### Function

&emsp; **compare** &mdash; return an integer comparing two values

### Synopsis

```
compare(a: any, b: any [, nullsMax: bool]) -> int64
```

### Description

The _compare_ function returns an integer comparing two values. The result will
be 0 if a is equal to b, +1 if a is greater than b, and -1 if a is less than b.
_compare_ differs from [comparison expressions](../expressions.md#comparisons) in that it will
work for any type (e.g., `compare(1, "1")`).

Values are compared via byte order.  Between values of type `string`, this is
equivalent to [C/POSIX collation](https://www.postgresql.org/docs/current/collation.html#COLLATION-MANAGING-STANDARD)
as found in other SQL databases such as Postgres.

`nullsMax` is an optional value (true by default) that determines whether `null`
is treated as the minimum or maximum value.

### Examples

```mdtest-spq
# spq
compare(a, b)
# input
{a:2,b:"1"}
# expected output
-1
```
