## Constants

Constants may be defined and assigned to a name with the syntax
```
const <id> = <expr>
```
where `<id>` is an [identifier](../syntax.md#identifiers)
and `<expr>` is a constant [expression](expressions.md)
that must evaluate to a constant at compile time and not reference any
runtime state such as `this` or a field of `this`.

A constant declaration must appear in the declaration section of a [scope](../syntax.md#scope).

A constant can be any expression, inclusive of subqueries and function calls, as
long as the expression evalautes to a compile-time constant.

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
