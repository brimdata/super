### Function

&emsp; **is** &mdash; test a value's type

### Synopsis
```
is(t: type) -> bool
is(val: any, t: type) -> bool
```

### Description

The _is_ function returns true if the argument `val` is of type `t`. If `val`
is omitted, it defaults to `this`.  The _is_ function is shorthand for `typeof(val)==t`.

### Examples

Test simple types:
```mdtest-spq
# spq
values {yes:is(<float64>),no:is(<int64>)}
# input
1.
# expected output
{yes:true,no:false}
```

Test for a given input's record type or "shape":
```mdtest-spq
# spq
values is(<{s:string}>)
# input
{s:"hello"}
# expected output
true
```

If you test a named type with its underlying type, the types are different,
but if you use the type name or typeof and under functions, there is a match:
```mdtest-spq
# spq
values is(<{s:string}>)
# input
{s:"hello"}::=foo
# expected output
false
```

```mdtest-spq
# spq
values is(<foo>)
# input
{s:"hello"}::=foo
# expected output
true
```

To test the underlying type, just use `==`:
```mdtest-spq
# spq
values under(typeof(this))==<{s:string}>
# input
{s:"hello"}::=foo
# expected output
true
```
