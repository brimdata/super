### Function

&emsp; **typeof** &mdash; the type of a value

### Synopsis

```
typeof(val: any) -> type
```

### Description

The _typeof_ function returns the [type](../../formats/sup.md#25-types) of
its argument `val`.  Types are first class so the returned type is
also a value.  The type of a type is type `type`.

### Examples

The types of various values:

```mdtest-spq
# spq
yield typeof(this)
# input
1
"foo"
10.0.0.1
[1,2,3]
{s:"foo"}
null
error("missing")
# expected output
<int64>
<string>
<ip>
<[int64]>
<{s:string}>
<null>
<error(string)>
```

The type of a type is type `type`:
```mdtest-spq
# spq
yield typeof(typeof(this))
# input
null
# expected output
<type>
```
