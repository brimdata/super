# WHERE

A `WHERE` clause has the form
```
WHERE <predicate>
```
where `<predicate>` is a Boolean-valued [expression](../expressions/index.md).

A WHERE clause is a component of [SELECT](select.md) that is applied
to the query's [input](from.md) removing each value for which 
`<predicate>` is false.

As in [PostgreSQL](https://www.postgresql.org/),
table and column references in the `WHERE` clause bind only to the
[input scope](select.md#input-scope).

XXX explain odd scoping of applying to input and output...

