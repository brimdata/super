## Conditionals

Conditional expressions compute a result from two or more possibilities
determined by Boolean predicates.

Conditionals can be written using SQL-style [CASE syntax](#case-expressions) or C-style
[ternary expressions](#ternary-conditional).

### Case Expressions

SQL-style `CASE` expressions have two forms.

The first form has the syntax
```
CASE <expr>
WHEN <expr-1> THEN <result-1>
[ WHEN <expr-2> THEN <result-2> ]
...
[ ELSE <else-result> ]
END
```
The expression `<expr>` is evaluated and compared with each subsequent
`WHEN` expression `<expr-1>`, `<expr-2>`, etc. until a match is found,
in which case, the corresponding expression `<result-n>` is evaluated for the match,
and that value becomes the result of the `CASE` expression.
If there is no match and an `ELSE` clause is present, the the result is
determined by the expression `<else-result>`.  Otherwise, the result is `null`.

The second form omits `<expr>` from above and has the syntax
```
CASE
WHEN <predicate-1> THEN <result-1>
[ WHEN <predicate-2> THEN <result-2> ]
...
[ ELSE <else-result> ]
END
```
Here, each `WHEN` expression must be Boolean-valued and
`<predicate-1>`, `<predicate-2>`, etc. are evaluated
in order until a true result is encountered,
in which case, the corresponding expression `<result-n>` is evaluated for the match,
and that value becomes the result of the `CASE` expression.
If there is no `true` result and an `ELSE` clause is present, the the result is
determined by the expression `<else-result>`.  Otherwise, the result is `null`.

If the predicate expressions are not Boolean valued, then an error results.
The error is reported at compile time if possible, but when input is dynamic and
the type cannot be statically determined, a [structured error](../types/error.md)
is generated at run time as the result of the conditional expression.

### Ternary Conditional

The ternary form follows the C language and has syntax
```
<predicate> ? <true-expr> : <false-expr>
```
where `<predicate>` is a Boolean-valued expression
and `<true-expr>` and `<false-expr>` are any [expressions](intro.md).
When `<predicate>` is true, then `<true-expr>` is evaluated and becomes
the result of the conditional expression; otherwise, `<false-expr>`
becomes the result.

If `<predicate>` is not a Boolean, then an error results.  The error
is reported at compile time if possible, but when input is dynamic and
the type cannot be statically determined, a [structured error](../types/error.md)
is generated at run time as the result of the conditional expression.

### Examples

---

_A simple ternary conditional_

```mdtest-spq
# spq
values (s=="foo") ? v : -v
# input
{s:"foo",v:1}
{s:"bar",v:2}
# expected output
1
-2
```

---

_The previous example as a CASE expression_

```mdtest-spq
# spq
values CASE WHEN s="foo" THEN v ELSE -v END
# input
{s:"foo",v:1}
{s:"bar",v:2}
# expected output
1
-2
```

---

_Ternary conditionals can be chained_

```mdtest-spq
# spq
values (s=="foo") ? v : (s=="bar") ? -v : v*v
# input
{s:"foo",v:1}
{s:"bar",v:2}
{s:"baz",v:3}
# expected output
1
-2
9
```

---

_The previous example as a CASE expression_

```mdtest-spq
# spq
values
  CASE s
  WHEN "foo" THEN v
  WHEN "bar" THEN -v
  ELSE v*v
  END
# input
{s:"foo",v:1}
{s:"bar",v:2}
{s:"baz",v:3}
# expected output
1
-2
9
```
