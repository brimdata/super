### Function

&emsp; **fields** &mdash; return the flattened path names of a record

### Synopsis

```
fields(r: record) -> [[string]]
```

### Description

The _fields_ function returns an array of string arrays of all the field names in record `r`.
A field's path name is representing by an array of strings since the dot
separator is an unreliable indicator of field boundaries as `.` itself
can appear in a field name.

`error("missing")` is returned if `r` is not a record.

### Examples

Extract the fields of a nested record:
```mdtest-spq
# spq
values fields(this)
# input
{a:1,b:2,c:{d:3,e:4}}
# expected output
[["a"],["b"],["c","d"],["c","e"]]
```

Easily convert to dotted names if you prefer:
```mdtest-spq
# spq
over fields(this) | values join(this,".")
# input
{a:1,b:2,c:{d:3,e:4}}
# expected output
"a"
"b"
"c.d"
"c.e"
```

A record is expected:
```mdtest-spq
# spq
values {f:fields(this)}
# input
1
# expected output
{f:error("missing")}
```
