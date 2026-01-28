# pass

[✅](../intro.md#data-order)&ensp; copy input values to output

## Synopsis

```
pass
```

## Description

The `pass` operator outputs a copy of each input value. It is typically used
with operators that handle multiple branches of the pipeline such as
[`fork`](fork.md) and [`join`](join.md).

## Examples

---

_Copy input to output_
```mdtest-spq
# spq
pass
# input
1
2
3
# expected output
1
2
3
```

---

_Copy each input value to three parallel pipeline branches and leave the values unmodified on one of them_
```mdtest-spq
# spq
fork
  ( pass )
  ( upper(this) )
  ( lower(this) )
| sort
# input
"HeLlo, WoRlD!"
# expected output
"HELLO, WORLD!"
"HeLlo, WoRlD!"
"hello, world!"
```
