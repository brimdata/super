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
```mdtest-command
echo '1 "a" 10.0.0.1' | super -z -c 'yield kind(this)' -
```

```mdtest-output
"primitive"
"primitive"
"primitive"
```

A complex value's kind is it's complex type category.  Try it on
these empty values of various complex types:
```mdtest-command
echo '{} [] |[]| |{}| 1((int64,string))' | super -z -c 'yield kind(this)' -
```

```mdtest-output
"record"
"array"
"set"
"map"
"union"
```

A Zed error has kind "error":
```mdtest-command
echo null | super -z -c 'yield kind(1/0)' -
```

```mdtest-output
"error"
```

A Zed type's kind is the kind of the type:
```mdtest-command
echo '<{s:string}>' | super -z -c 'yield kind(this)' -
```

```mdtest-output
"record"
```
