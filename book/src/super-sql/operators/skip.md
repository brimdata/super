# skip

[✅](../intro.md#data-order)&ensp; skip leading values of input sequence

## Synopsis

```
skip <const-expr>
```

## Description

The `skip` operator skips the first N values from its input. N is given by
`<const-expr>`, a compile-time constant expression that evaluates to a positive
integer.

## Examples

---

_Skip the first two values of an arbitrary sequence_
```mdtest-spq
# spq
skip 2
# input
1
"foo"
[1,2,3]
# expected output
[1,2,3]
```
