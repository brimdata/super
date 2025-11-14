## Exists

The `exists` operator is Boolean-valued function that tests whether
a subquery has a non-empty result and has the form
```
exists ( <query> )
```
where `<query>` is any [query](../queries.md).

It is a syntactic shortcut for
```
len([<query>]) != 0
```

### Examples

---

_Simple example showing true for non-empty result_

```mdtest-spq
# spq
values exists (values 1,2,3)
# input
null
# expected output
true
```

---

_EXISTS is typically used with correlated subqueries but they are not yet supported_

```mdtest-spq
# spq
let Orders = (values {product:"widget",customer_id:1})
SELECT 'there are orders in the system' as s
WHERE EXISTS (
    SELECT 1
    FROM Orders
)
# input
null
# expected output
{s:"there are orders in the system"}
```
