### Function

&emsp; **fields** &mdash; return the flattened path names of a record

### Synopsis

```
fields(r: record) -> [[string]]
```

### Description

The `fields` function returns an array of string arrays of all the field names in record `r`.
A field's path name is represented by an array of strings since the dot
separator is an unreliable indicator of field boundaries as `.` itself
can appear in a field name.

`error("missing")` is returned if `r` is not a record.

### Examples

---

_Extract the fields of a nested record_

```mdtest-spq
# spq
values fields(this)
# input
{a:1,b:2,c:{d:3,e:4}}
# expected output
[["a"],["b"],["c","d"],["c","e"]]
```

---

_Easily convert to dotted names if you prefer_

```mdtest-spq
# spq
unnest fields(this) | values join(this,".")
# input
{a:1,b:2,c:{d:3,e:4}}
# expected output
"a"
"b"
"c.d"
"c.e"
```

---

_A record is expected_

```mdtest-spq
# spq
values {f:fields(this)}
# input
1
# expected output
{f:error("missing")}
```
