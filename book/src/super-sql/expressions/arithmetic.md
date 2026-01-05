## Arithmetic

Arithmetic operations (`*`, `/`, `%`, `+`, `-`) follow customary syntax
and semantics and are left-associative with multiplication and division having
precedence over addition and subtraction.  `%` is the modulo operator.

### Unary Sign

Any number may be signed with a unary operator having the form:
```
- <expr>
```
and
```
+ <expr>
```
where `<expr>` is any [expression](intro.md) that results in a number type.

### Example

---

```mdtest-spq
# spq
values 2*3+1, 11%5, 1/0, +1, -1
# input

# expected output
7
1
error("divide by zero")
1
-1
```
