## Containment

A containment expression expression tests for the existence of
a value in another value and has the form
```
<item> in <target>
```
where `<item>` and `<target>` are [expressions](intro.md).

The result is Boolean-valued and is true
and is true if the `<item>` expression results in a value that
appears somewhere in the `<target>` expression as an exact match of the item.

In contrast to SQL's `IN` operator, the right-hand side can be any value and when the
`<item>` and `<target>` are equal, the result of `in` is true, e.g.,
```
1 in 1
```
is semantically valid and results in true.

The inverse of `in` has the syntax
```
<item> not in <target>
```
and is true when `<item>` is not contained in the `<target>`.

>[!NOTE]
> The `in` operator currently does not support SQL NULL semantics in that
> `1 not in [2,NULL]` is false instead of NULL.  This will be fixed
> in a future version.

When the `<target>` is a non-array [subquery](subqueries.md), it is coerced to an
[array subquery](subqueries.md#array-subqueries) and the `in` expression is evaluated
on the array result of the subquery.

### Examples

---

_Test for the value `1` in the input values_

```mdtest-spq
# spq
where 1 in this
# input
{a:[1,2]}
{b:{c:3}}
{d:{e:1}}
# expected output
{a:[1,2]}
{d:{e:1}}
```

---

_Test against a predetermined values with a literal array_

```mdtest-spq
# spq
unnest accounts | where id in [1,2]
# input
{accounts:[{id:1},{id:2},{id:3}]}
# expected output
{id:1}
{id:2}
```

---

_Complex values are recursively searched for containment_

```mdtest-spq
# spq
where {s:"foo"} in this
# input
{s:"foo"}
{s:"foo",t:"bar"}
{a:{s:"foo"}}
[1,{s:"foo"},2]
# expected output
{s:"foo"}
{a:{s:"foo"}}
[1,{s:"foo"},2]
```

---

_Subqueries work the same whether they are standard style or array style_

```mdtest-spq
# spq
let vals = (values 1,2,3)
values {a:(this in vals), b:(this in [vals])}
# input
1
2
4
# expected output
{a:true,b:true}
{a:true,b:true}
{a:false,b:false}
```
