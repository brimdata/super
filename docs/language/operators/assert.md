### Operator

&emsp; **assert** &mdash; evaluate an assertion

### Synopsis

```
assert <expr>
```
### Description

The `assert` operator evaluates the Boolean expression `<expr>` for each
input value, yielding its input value if `<expr>` evaluates to true or a
structured error if it does not.

### Examples

```mdtest-command
echo {a:1} | super -z -c 'assert a > 0' -
```

```mdtest-output
{a:1}
```

```mdtest-command
echo {a:-1} | super -z -c 'assert a > 0' -
```

```mdtest-output
error({message:"assertion failed",expr:"a > 0",on:{a:-1}})
```
