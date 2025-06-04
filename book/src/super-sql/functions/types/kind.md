### Function

&emsp; **kind** &mdash; return a value's type category

### Synopsis

```
kind(val: any) -> string
```

### Description

The `kind` function returns the category of the type of `v` as a string,
e.g., "record", "set", "primitive", etc.  If `v` is a type value,
then the type category of the referenced type is returned.

### Examples

---

_A primitive value's kind is "primitive"_

```mdtest-spq
# spq
values kind(this)
# input
1
"a"
10.0.0.1
# expected output
"primitive"
"primitive"
"primitive"
```

---

_A complex value's kind is its complex type category.  Try it on
these empty values of various complex types_

```mdtest-spq
# spq
values kind(this)
# input
{}
[]
|[]|
|{}|
1::(int64|string)
# expected output
"record"
"array"
"set"
"map"
"union"
```

---

_An error has kind "error"_

```mdtest-spq
# spq
values kind(1/this)
# input
0
# expected output
"error"
```

---

_A type's kind is the kind of the type_

```mdtest-spq
# spq
values kind(this)
# input
<{s:string}>
# expected output
"record"
```
