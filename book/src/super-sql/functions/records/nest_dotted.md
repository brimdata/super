### Function

&emsp; **nest_dotted** &mdash; transform fields in a record with dotted names
<<<<<<< HEAD
to nested records
=======
to nested records.
>>>>>>> 5c89084a6 ([book])

### Synopsis

```
nest_dotted(val: record) -> record
```

### Description

The `nest_dotted` function returns a copy of `val` with all dotted field names
converted into nested records.

### Examples

---

```mdtest-spq
# spq
values nest_dotted(this)
# input
{"a.b.c":"foo"}
# expected output
{a:{b:{c:"foo"}}}
```
