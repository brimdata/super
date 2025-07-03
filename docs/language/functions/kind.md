### Function

&emsp; **kind** &mdash; return a value's type category

### Synopsis

```
kind(val: any) -> string
```

### Description

The _kind_ function returns the category of the type of `v` as a string,
e.g., "record", "set", "primitive", etc.  If `v` is a type value,
then the type category of the referenced type is returned.

#### Example:

A primitive value's kind is "primitive":
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

A complex value's kind is its complex type category.  Try it on
these empty values of various complex types:
```mdtest-spq
# spq
values kind(this)
# input
{}
[]
|[]|
|{}|
1(int64|string)
# expected output
"record"
"array"
"set"
"map"
"union"
```

An error has kind "error":
```mdtest-spq
# spq
values kind(1/this)
# input
0
# expected output
"error"
```

A type's kind is the kind of the type:
```mdtest-spq
# spq
values kind(this)
# input
<{s:string}>
# expected output
"record"
```
