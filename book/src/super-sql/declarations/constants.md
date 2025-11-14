## Constants

Constants are declared with the syntax
```
const <id> = <expr>
```
where `<id>` is an [identifier](../queries.md#identifiers)
and `<expr>` is a constant [expression](../expressions/intro.md)
that must evaluate at compile time without referencing any
runtime state such as `this` or a field of `this`.

Constant declarations must appear in the declaration section of a
[scope](../queries.md#scope).

A constant can be any expression, inclusive of subqueries and function calls, as
long as the expression evaluates to a compile-time constant.

### Examples

---

_A simple declaration for the identifier `PI`_

```mdtest-spq
# spq
const PI=3.14159
values 2*PI*r
# input
{r:5}
{r:10}
# expected output
31.4159
62.8318
```

---

_A constant as a subquery that is independent of external input_

```mdtest-spq
# spq
const ABC = [
  values 'a', 'b', 'c'
  | upper(this)
]
values ABC
# input
null
# expected output
["A","B","C"]
```
