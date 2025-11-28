### Operator

[✅](../intro.md#data-order)&emsp; **drop** &mdash; drop fields from record values

### Synopsis

```
drop <field> [, <field> ...]
```
### Description

The `drop` operator removes one or more fields from records in the input sequence
and copies the modified records to its output.  If a field to be dropped
is not present, then no effect for the field occurs.  In particular,
non-record values are copied unmodified.

### Examples

---

_Drop a field_
```mdtest-spq
# spq
drop b
# input
{a:1,b:2,c:3}
# expected output
{a:1,c:3}
```

---

_Non-record values are copied to output_
```mdtest-spq
# spq
drop a,b
# input
1
{a:1,b:2,c:3}
# expected output
1
{c:3}
```
