# count

[✅](../intro.md#data-order)&ensp; emit records containing a running count of input values

>[!TIP]
> For a final count of all input values, see the [count](../aggregates/count.md) aggregate function.

## Synopsis

```
count [ <record-expr> ]
```
## Description

The `count` operator produces records that include a running count of its input values.

When `<record-expr>` is present, it must be a
[record expression](../types/record.md#record-expressions) in which the
rightmost element is the name of a field to hold the numeric count and any
preceding elements are evaluated on the input value.

If the optional `<record-expr>` is absent, the output record is created with a
[derived field name](../types/record.md#derived-field-names) resulting in the
equivalent of `count {that:this,count}`.

## Examples

---

_A running count alongside complete copies of input values_
```mdtest-spq {data-layout="stacked"}
# spq
count
# input
{foo:"bar",a:true}
{foo:"baz",b:false}
# expected output
{that:{foo:"bar",a:true},count:1}
{that:{foo:"baz",b:false},count:2}
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
{c:1}
{c:2}
{c:3}
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
{foo:"bar",a:true,c:1}
{foo:"baz",b:false,c:2}
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
{third_foo_char:"r",c:1}
{third_foo_char:"z",c:2}
```
