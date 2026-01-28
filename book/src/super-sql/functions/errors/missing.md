# missing

test for the "missing" error

## Synopsis

```
missing(val: any) -> bool
```

## Description

The `missing` function returns true if its argument is `error("missing")`
and false otherwise.

This function is often used to test if certain fields do not appear as
expected in a record, e.g., `missing(a)` is true either when `this` is not a record
or when `this` is a record and the field `a` is not present in `this`.

It's also useful for shaping messy data when applying conditional logic based on the
absence of certain fields:
```
switch
  case missing(a) ( ... )
  case missing(b) ( ... )
  default ( ... )
```

## Examples

---

```mdtest-spq
# spq
values {yes:missing(bar),no:missing(foo)}
# input
{foo:10}
# expected output
{yes:true,no:false}
```

---

```mdtest-spq
# spq
values {yes:missing(foo.baz),no:missing(foo.bar)}
# input
{foo:{bar:"value"}}
# expected output
{yes:true,no:false}
```

---

```mdtest-spq
# spq
values {yes:missing(bar+1),no:missing(foo+1)}
# input
{foo:10}
# expected output
{yes:true,no:false}
```

---

```mdtest-spq
# spq
values missing(bar)
# input
1
# expected output
true
```

---

```mdtest-spq
# spq
values missing(x)
# input
{x:error("missing")}
# expected output
true
```
