# is

test a value's type

## Synopsis

```
is(val: any, t: type) -> bool
```

## Description

The `is` function returns true if the argument `val` is of type `t`.
The _is_ function is shorthand for `typeof(val)==t`.

## Examples

---

_Test simple types_

```mdtest-spq
# spq
values {yes:is(this, <float64>),no:is(this, <int64>)}
# input
1.
# expected output
{yes:true,no:false}
```

---

_Test for a given input's record type or "shape"_

```mdtest-spq
# spq
values is(this, <{s:string}>)
# input
{s:"hello"}
# expected output
true
```

---

_If you test a named type with its underlying type, the types are different,
but if you use the type name or typeof and under functions, there is a match_

```mdtest-spq
# spq
values is(this, <{s:string}>)
# input
{s:"hello"}::=foo
# expected output
false
```

---

```mdtest-spq
# spq
values is(this, <foo>)
# input
{s:"hello"}::=foo
# expected output
true
```

---

_To test the underlying type, just use `==`_

```mdtest-spq
# spq
values under(typeof(this))==<{s:string}>
# input
{s:"hello"}::=foo
# expected output
true
```
