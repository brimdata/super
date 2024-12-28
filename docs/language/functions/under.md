### Function

&emsp; **under** &mdash; the underlying value

### Synopsis

```
under(val: any) -> any
```

### Description

The _under_ function returns the value underlying the argument `val`:
* for unions, it returns the value as its elemental type of the union,
* for errors, it returns the value that the error wraps,
* for types, it returns the value typed as `typeunder()` indicates; otherwise,
* it returns `val` unmodified.

### Examples

Unions are unwrapped:
```mdtest-command
echo '1((int64,string)) "foo"((int64,string))' |
  super -z -c 'yield this' -

echo '1((int64,string)) "foo"((int64,string))' |
  super -z -c 'yield under(this)' -
```

```mdtest-output
1((int64,string))
"foo"((int64,string))
1
"foo"
```

Errors are unwrapped:
```mdtest-command
echo 'error("foo") error({err:"message"})' | super -z -c 'yield this' -
echo 'error("foo") error({err:"message"})' | super -z -c 'yield under(this)' -
```

```mdtest-output
error("foo")
error({err:"message"})
"foo"
{err:"message"}
```

Values of named types are unwrapped:
```mdtest-command
echo '80(port=uint16)' | super -z -c 'yield this' -
echo '80(port=uint16)' | super -z -c 'yield under(this)' -
```

```mdtest-output
80(port=uint16)
80(uint16)
```
Values that are not wrapped are unmodified:
```mdtest-command
echo '1 "foo" <int16> {x:1}' | super -z -c 'yield under(this)' -
```

```mdtest-output
1
"foo"
<int16>
{x:1}
```
