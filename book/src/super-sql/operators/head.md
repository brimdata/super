# head

[✅](../intro.md#data-order)&ensp; copy leading values of input sequence

## Synopsis

```
head [ <const-expr> ]
limit [ <const-expr> ]
```
## Description

The `head` operator copies the first N values from its input to its output and ends
the sequence thereafter. N is given by `<const-expr>`, a compile-time
constant expression that evaluates to a positive integer. If `<const-expr>`
is not provided, the value of N defaults to `1`.

For compatibility with other pipe SQL dialects,
`limit` is an alias for the `head` operator.

## Examples

---

_Grab first two values of arbitrary sequence_
```mdtest-spq
# spq
head 2
# input
1
"foo"
[1,2,3]
# expected output
1
"foo"
```

---

_Grab first two values of arbitrary sequence, using a different representation of two_
```mdtest-spq
# spq
const ONE = 1
limit ONE+1
# input
1
"foo"
[1,2,3]
# expected output
1
"foo"
```

---

_Grab the first record of a record sequence_
```mdtest-spq
# spq
head
# input
{a:"hello"}
{a:"world"}
# expected output
{a:"hello"}
```
