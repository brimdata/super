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
* for named values, it returns the value with the name removed,
* for type values, it removes the named type if one exists; otherwise,
* it returns `val` unmodified.

### Examples

Unions are unwrapped:
```mdtest-spq
# spq
values this
# input
1::(int64|string)
"foo"::(int64|string)
# expected output
1::(int64|string)
"foo"::(int64|string)
```

```mdtest-spq
# spq
values under(this)
# input
1::(int64|string)
"foo"::(int64|string)
# expected output
1
"foo"
```

Errors are unwrapped:
```mdtest-spq
# spq
values this
# input
error("foo")
error({err:"message"})
# expected output
error("foo")
error({err:"message"})
```

```mdtest-spq
# spq
values under(this)
# input
error("foo")
error({err:"message"})
# expected output
"foo"
{err:"message"}
```

Values of named types are unwrapped:
```mdtest-spq
# spq
values this
# input
80::(port=uint16)
# expected output
80::(port=uint16)
```

```mdtest-spq
# spq
values under(this)
# input
80::(port=uint16)
# expected output
80::uint16
```

Values that are not wrapped are unmodified:
```mdtest-spq
# spq
values under(this)
# input
1
"foo"
<int16>
{x:1}
# expected output
1
"foo"
<int16>
{x:1}
```
