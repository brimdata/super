## Conditional

A conditional expression has the form
```
<boolean> ? <expr> : <expr>
```
The `<boolean>` expression is evaluated and must have a result of type `bool`.
If not, an error results.

If the result is true, then the first `<expr>` expression is evaluated and becomes
the result.  Otherwise, the second `<expr>` expression is evaluated and
becomes the result.

For example,
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

Conditional expressions can be chained, providing behavior equivalent to
"else if" as appears in other languages.

For example,
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
