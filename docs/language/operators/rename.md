### Operator

&emsp; **rename** &mdash; change the name of record fields

### Synopsis
```
rename <newfield>:=<oldfield> [, <newfield>:=<oldfield> ...]
```
### Description

The `rename` operator changes the names of one or more fields
in the input records from the right-hand side name to the left-hand side name
for each assignment listed.  When `<oldfield>` references a field that does not
exist, there is no effect and the input is copied to the output.

Non-record inputs are copied to the output without modification.

Each `<field>` must be a field reference as a dotted path and the old name
and new name must refer to the same record in the case of nested records.
That is, the dotted path prefix before the final field name must be the
same on the left- and right-hand sides.  To perform more sophisticated
renaming of fields, you can use cut/put or record literals.

If a rename operation conflicts with an existing field name, then the
offending record is wrapped in a structured error along with an error message
and the error is emitted.

### Examples

_A simple rename_
```mdtest-spq
# spq
rename c:=b
# input
{a:1,b:2}
# expected output
{a:1,c:2}
```

_Nested rename_
```mdtest-spq
# spq
rename r.a:=r.b
# input
{a:1,r:{b:2,c:3}}
# expected output
{a:1,r:{a:2,c:3}}
```

_Trying to mutate records with rename produces a compile-time error_
```mdtest-spq fails {data-layout="stacked"}
# spq
rename w:=r.b
# input
{a:1,r:{b:2,c:3}}
# expected output
left-hand side and right-hand side must have the same depth (w vs r.b) at line 1, column 8:
rename w:=r.b
       ~~~~~~
```

_Record literals can be used instead of rename for mutation_
```mdtest-spq
# spq
values {a,r:{c:r.c},w:r.b}
# input
{a:1,r:{b:2,c:3}}
# expected output
{a:1,r:{c:3},w:2}
```

_Alternatively, mutations can be more generic and use drop_
```mdtest-spq
# spq
values {a,r,w:r.b} | drop r.b
# input
{a:1,r:{b:2,c:3}}
# expected output
{a:1,r:{c:3},w:2}
```

_Duplicate fields create structured errors_
```mdtest-spq {data-layout="stacked"}
# spq
rename a:=b
# input
{b:1}
{a:1,b:1}
{c:1}
# expected output
{a:1}
error({message:"rename: duplicate field: \"a\"",on:{a:1,b:1}})
{c:1}
```
