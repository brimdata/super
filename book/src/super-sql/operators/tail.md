### Operator

&emsp; **tail** &mdash; copy trailing values of input sequence

### Synopsis

```
tail [ <const-expr> ]
```
### Description

The `tail` operator copies the last N from its input to its output and ends
the sequence thereafter. N is given by `<const-expr>`, a compile-time
constant expression that evaluates to a positive integer. If `<const-expr>`
is not provided, the value of N defaults to `1`.

### Examples

---

_Grab last two values of arbitrary sequence_
```mdtest-spq
# spq
tail 2
# input
1
"foo"
[1,2,3]
# expected output
"foo"
[1,2,3]
```

---

_Grab last two values of arbitrary sequence, using a different representation of two_
```mdtest-spq
# spq
tail 1+1
# input
1
"foo"
[1,2,3]
# expected output
"foo"
[1,2,3]
```

---

_Grab the last record of a record sequence_
```mdtest-spq
# spq
tail
# input
{a:"hello"}
{b:"world"}
# expected output
{b:"world"}
```
