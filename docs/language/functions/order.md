### Function

&emsp; **order** &mdash; reorder record fields

### Synopsis

```
order(val: any, t: type) -> any
```

### Description

The _order_ function changes the order of fields in the input value `val`
to match the order of records in type `t`. Ordering is useful when the
input is in an unordered format (such as JSON), to ensure that all records
have the same known order.

If `val` is a record (or if any of its nested values is a record):
* order passes through "extra" fields not present in the type value,
* extra fields in the input are added to the right-hand side, ordered lexicographically,
* missing fields are ignored, and
* types of leaf values are ignored, i.e., there is no casting.

Note that lexicographic order for fields in a record can be achieved with
the empty record type, i.e.,
```
order(val, <{}>)
```
{{% tip "Tip" %}}

Many users seeking the functionality of `order` prefer to use the
[`shape` function](./shape.md) which applies the `order`, [`cast`](./cast.md),
and [`fill`](./fill.md) functions simultaneously on a record.

{{% /tip %}}

{{% tip "Note" %}}

[Record expressions](../expressions.md#record-expressions) can also be used to
reorder fields without specifying types ([example](../shaping.md#order)).

{{% /tip %}}

### Examples

_Order a record_
```mdtest-command
echo '{b:"foo", a:1}' | super -z -c 'order(this, <{a:int64,b:string}>)' -
```
produces
```mdtest-output
{a:1,b:"foo"}
```
_Order fields lexicographically_
```mdtest-command
echo '{c:0, a:1, b:"foo"}' | super -z -c 'order(this, <{}>)' -
```
produces
```mdtest-output
{a:1,b:"foo",c:0}
```

TBD: fix this bug or remove example...

_Order an array of records_
```mdtest-command-skip
echo '[{b:1,a:1},{a:2,b:2}]' | super -z -c 'order(this, <[{a:int64,b:int64}]>)' -
```
produces
```mdtest-output-skip
[{a:1,b:1},{a:2,b:2}]
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
