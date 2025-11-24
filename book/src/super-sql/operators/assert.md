### Operator

[✅](../intro.md#data-order)&emsp; **assert** &mdash; test a predicate and produce errors on failure

### Synopsis

```
assert <expr>
```
### Description

The `assert` operator evaluates the Boolean expression `<expr>` for each
input value, producing its input value if `<expr>` evaluates to true or a
structured error if it does not.

### Examples

---

```mdtest-spq {data-layout="stacked"}
# spq
assert a > 0
# input
{a:1}
{a:-1}
# expected output
{a:1}
error({message:"assertion failed",expr:"a > 0",on:{a:-1}})
```
