### Function

&emsp; **fill** &mdash; add null values for missing record fields

### Synopsis

```
fill(val: any, t: type) -> any
```

### Description

The _fill_ function adds to the input record `val` any fields that are
present in the output type `t` but not in the input.  Such fields are added
after the fields already present in `t` and in the order they appear in the
input record.

Filled fields are added with a `null` value.  Filling is useful when
you want to be sure that all fields in a schema are present in a record.

If `val` is not a record, it is returned unmodified.

{{% tip "Tip" %}}

Many users seeking the functionality of `fill` prefer to use the
[`shape` function](./shape.md) which applies the `fill`, [`cast`](./cast.md),
and [`order`](./order.md) functions simultaneously on a record.

{{% /tip %}}

### Examples

_Fill a record_
```mdtest-command
echo '{a:1}' | super -z -c 'fill(this, <{a:int64,b:string}>)' -
```
produces
```mdtest-output
{a:1,b:null(string)}
```

_Fill an array of records_
```mdtest-command
echo '[{a:1},{a:2}]' | super -z -c 'fill(this, <[{a:int64,b:int64}]>)' -
```
produces
```mdtest-output
[{a:1,b:null(int64)},{a:2,b:null(int64)}]
```

_Non-records are returned unmodified_
```mdtest-command
echo '10.0.0.1 1 "foo"' | super -z -c 'fill(this, <{a:int64,b:int64}>)' -
```
produces
```mdtest-output
10.0.0.1
1
"foo"
```
