### Function

&emsp; **error** &mdash; wrap a value as an error

### Synopsis

```
error(val: any) -> error
```

### Description

The `error` function returns an error version of any value.
It wraps the value `val` to turn it into an error type providing
a means to create structured and stacked errors.

### Examples

---

_Wrap a record as a structured error_

```mdtest-spq {data-layout="stacked"}
# spq
values error({message:"bad value", value:this})
# input
{foo:"foo"}
# expected output
error({message:"bad value",value:{foo:"foo"}})
```

---

_Wrap any value as an error_

```mdtest-spq
# spq
values error(this)
# input
1
"foo"
[1,2,3]
# expected output
error(1)
error("foo")
error([1,2,3])
```

---

_Test if a value is an error and show its type "kind"_

```mdtest-spq {data-layout="stacked"}
# spq
values {this,err:is_error(this),kind:kind(this)}
# input
error("exception")
"exception"
# expected output
{this:error("exception"),err:true,kind:"error"}
{this:"exception",err:false,kind:"primitive"}
```

---

_Comparison of a missing error results in a missing error even if they
are the same missing errors so as to not allow field comparisons of two
missing fields to succeed_

```mdtest-spq
# spq
badfield:=x | values badfield==error("missing")
# input
{}
# expected output
error("missing")
```
