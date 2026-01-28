# Pragmas

Pragmas control various language features and appear in a declaration block
so their effect is lexically scoped.  They have the form
```
pragma <id> [ = <expr> ]
```
where `<id>` is an [identifier](../queries.md#identifiers)
and `<expr>` is a constant [expression](../expressions/intro.md)
that must evaluate at compile time without referencing any
runtime state such as [this](../intro.md#pipe-scoping) or a field of `this`.

If `<expr>` is omitted, it defaults to `true`.

Pragmas must appear in the declaration section of a [scope](../queries.md#scope).

## List of Pragmas

Currently, there are two supported pragmas.

* `index_base` - controls whether [index expressions](../expressions/index.md) and
    [slice expressions](../expressions/slices.md) are 0-based or 1-based.
    * `0` for zero-based indexing
    * `1` for one-based indexing
* `pg` - controls the precedence of scoping for GROUP BY clauses in SQL operators
    * `false` to follow Google SQL semantics of resolving identifiers first from column aliases then from the input table
    * `true` to follow PostgreSQL semantics of resolving identifiers first from the input table then from the column aliases

## Example

---

_Controlling indexing and slicing_

```mdtest-spq
# spq
pragma index_base = 1
values {
  a: this[2:3],
  b: (
    pragma index_base = 0
    values this[0]
  )
}
# input
"bar"
[1,2,3]
# expected output
{a:"a",b:error("missing")}
{a:[2],b:1}
```
