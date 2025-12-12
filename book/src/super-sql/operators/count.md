### Operator

[✅](../intro.md#data-order)&emsp; **count** &mdash; emit records containing a running count of input values

>[!TIP]
> For a final count of all input values, see the [count](../aggregates/count.md) aggregate function.

### Synopsis

```
count [ <record-expr> ]
```
### Description

The `count` operator produces a running count of input values as a field in
an output record, including some or all of parts of input values in the output.

If the optional `<record-expr>` is absent, the output record contains the
input value in an element of [derived field name](../types/record.md#derived-field-names)
`that` and a field of name `count` that contains the numeric count.

When `<record-expr>` is present, it must be a
[record expression](../types/record.md#record-expressions) in which the
rightmost element is the name of a field to hold the numeric count. Any
preceding elements determine what parts of the input value to include in the
output record, similar to how a record expression may be used with the
[values](values.md) operator.

### Examples

---

_A running count alongside complete copies of input values_
```mdtest-spq {data-layout="stacked"}
# spq
count
# input
{foo:"bar",a:true}
{foo:"baz",b:false}
# expected output
{that:{foo:"bar",a:true},count:1::uint64}
{that:{foo:"baz",b:false},count:2::uint64}
```

---

_A running count in specified named field, ignoring input values_
```mdtest-spq
# spq
count {c}
# input
"a"
"b"
"c"
# expected output
{c:1::uint64}
{c:2::uint64}
{c:3::uint64}
```

---

_Spreading a complete input record alongside a running count_
```mdtest-spq
# spq
count {...this,c}
# input
{foo:"bar",a:true}
{foo:"baz",b:false}
# expected output
{foo:"bar",a:true,c:1::uint64}
{foo:"baz",b:false,c:2::uint64}
```

---

_Preserving select parts of input values alongside a running count_
```mdtest-spq
# spq
count {third_foo_char:foo[2:3],c}
# input
{foo:"bar",a:true}
{foo:"baz",b:false}
# expected output
{third_foo_char:"r",c:1::uint64}
{third_foo_char:"z",c:2::uint64}
```
