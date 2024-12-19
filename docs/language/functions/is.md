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
```mdtest-command
echo '1.' | super -z -c 'yield {yes:is(<float64>),no:is(<int64>)}' -
```

```mdtest-output
{yes:true,no:false}
```

Test for a given input's record type or "shape":
```mdtest-command
echo '{s:"hello"}' | super -z -c 'yield is(<{s:string}>)' -
```

```mdtest-output
true
```
If you test a named type with it's underlying type, the types are different,
but if you use the type name or typeunder function, there is a match:
```mdtest-command
echo '{s:"hello"}(=foo)' | super -z -c 'yield is(<{s:string}>)' -
echo '{s:"hello"}(=foo)' | super -z -c 'yield is(<foo>)' -
```

```mdtest-output
false
true
```

To test the underlying type, just use `==`:
```mdtest-command
echo '{s:"hello"}(=foo)' | super -z -c 'yield typeunder(this)==<{s:string}>' -
```

```mdtest-output
true
```
