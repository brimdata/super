### Operator

&emsp; **assert** &mdash; evaluate an assertion

### Synopsis

```
assert <expr>
```
### Description

The `assert` operator evaluates the Boolean expression `<expr>` for each
input value, returning its input value if `<expr>` evaluates to true or a
structured error if it does not.

### Examples

```mdtest-spq
# spq
assert a > 0
# input
{a:1}
# expected output
{a:1}
```

```mdtest-spq {data-layout="stacked"}
# spq
assert a > 0
# input
{a:-1}
# expected output
error({message:"assertion failed",expr:"a > 0",on:{a:-1}})
```
