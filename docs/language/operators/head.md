### Operator

&emsp; **head** &mdash; copy leading values of input sequence

### Synopsis

```
head [ <const-expr> ]
```
### Description

The `head` operator copies the first N values from its input to its output and ends
the sequence thereafter. N is given by `<const-expr>`, a compile-time
constant expression that evaluates to a positive integer. If `<const-expr>`
is not provided, the value of N defaults to `1`.

### Examples

_Grab first two values of arbitrary sequence_
```mdtest-command
echo '1 "foo" [1,2,3]' | super -z -c 'head 2' -
```

```mdtest-output
1
"foo"
```

_Grab first two values of arbitrary sequence, using a different representation of two_
```mdtest-command
echo '1 "foo" [1,2,3]' | super -z -c 'head 1+1' -
```

```mdtest-output
1
"foo"
```

_Grab the first record of a record sequence_
```mdtest-command
echo '{a:"hello"}{b:"world"}' | super -z -c head -
```

```mdtest-output
{a:"hello"}
```
