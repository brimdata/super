## Arithmetic

Arithmetic operations (`*`, `/`, `%`, `+`, `-`) follow customary syntax
and semantics and are left-associative with multiplication and division having
precedence over addition and subtraction.  `%` is the modulo operator.

For example,
```mdtest-spq
# spq
values 2*3+1, 11%5, 1/0, "foo"+"bar"
# input
null
# expected output
7
1
error("divide by zero")
"foobar"
```

