## Subqueries

A subquery is a [query](../syntax.md) embedded in an [expression](intro.md).

When the expression containing the subquery is evaluated, the query is run with
an input consisting of a single value equal to the value being evaluated
by the expression.

The syntax for a subquery is simply a query in paranetheses as in
```
( <query> )
```
where `<query>` is any query, e.g., the query
```
values {s:(values "hello, world" | upper(this))}
```
results in in the value `{s:"HELLO, WORLD"}`.

Except for subqueries appearing as the right-hand side of
an [in](containment.md) operator, the result of a subquery must be a single value. 
When multiple values are generated, an error is produced.

For the [in](containment.md) operator, any subquery on the right-hand side is
always treated as an [array subquery](#array-subqueries), thus
providing compatibility with SQL syntax.

### Array Subqueries

When multiple values are expected, an array subquery can be used to group the
multi-valued result into a single-valued array.

The syntax for an array subquery is simply a query in square brackets as in
```
[ <query> ]
```
where `<query>` is any query, e.g., the query
```
values {a:[values 1,2,3 | values this+1]}
```
results in the value `{a:[2,3,4]}`.

An array subquery is shorthand for
```
( <query> | collect(this) )
```
e.g., the array subquery above could also be rewritten as
```
values {a:(values 1,2,3 | values this+1 | collect(this))}
```


### Independent Subqueries

A subquery that depends on its input as described above is called a _dependent subquery_.

When the subquery ignores its input value, e.g., when it begins with 
a [from](../operators/from.md) operator, then they query is called an _independent subquery_.

For efficiency, the system materializes independent subqueries so that they are evaluated
just once.

For example, this query
```
let input = (values 1,2,3)
values 3,4
| values {that:this,count:(from input | count())}
```
evaluates the subquery `from input | count()` just once and materializes the result.
Then, for each input value `3` and `4`, the result is emitted, e.g.,
```
{that:3,count:3::uint64}
{that:4,count:3::uint64}
```

### SQL Subqueries

When a subquery appears within a SQL operator, relational scope is active
and references to table aliases and columns may reach a scope that is outside
of the subquery.  In this case, the subquery is a
[correlated subquery](https://en.wikipedia.org/wiki/Correlated_subquery).

### Named Subqueries

Queries declared as named queries may be referenced in expressions without
the ...

XXX operators "called" from expressions

### Examples

---

_Operate on arrays with values shortcuts and arrange answers into a record_

```mdtest-spq {data-layout="stacked"}
# spq
values {
    squares:[unnest this | this*this],
    roots:[unnest this | round(sqrt(this)*100)*0.01]
}
# input
[1,2,3]
[4,5]
# expected output
{squares:[1,4,9],roots:[1.,1.41,1.73]}
{squares:[16,25],roots:[2.,2.24]}
```

---


_Multi-valued subqueries emit an error_

```mdtest-spq {data-layout="stacked"}
# spq
values (values 1,2)
# input
null
# expected output
error("query expression produced multiple values (consider [(subquery)])")
```
---

_Multi-valued subqueries can be invoked as an array subquery_

```mdtest-spq
# spq
values [values 1,2]
# input
null
# expected output
[1,2]
```

---

_Right-hand side of "in" operator is always an array subquery_

```mdtest-spq
# spq
let data = (values {x:1},{x:2})
where this in (select x from data)
# input
1
2
3
# expected output
1
2
```


---

_Independent subqueries in SQL operators are supported while correlated subqueries are not_

```mdtest-spq
# spq
let input = (values {x:1},{x:2},{x:3})
select *
from input
where x >= (select avg(x) from input)  
# input
null
# expected output
{x:2}
{x:3}
```

---

_Correlated subqueries in SQL operators not yet supported_

```mdtest-spq
# spq
XXX
# input
XXX
# expected output
XXX
```

---

XXX old examples


```mdtest-spq
# spq
unnest this into (
  sort this | collect(this)
)
# input
[3,2,1]
[4,1,7]
[1,2,3]
# expected output
[1,2,3]
[1,4,7]
[1,2,3]
```

## Lateral Expressions

Lateral subqueries can also appear in expression context using the
parenthesized form:
```
( over <expr> [, <expr>...] [with <var>=<expr> [, ... <var>[=<expr>]] | <lateral> )
```

> _The parentheses disambiguate a lateral expression from a [lateral pipeline operator](operators/over.md)._

This form must always include a [lateral scope](#lateral-scope) as indicated by `<lateral>`.

The lateral expression is evaluated by evaluating each `<expr>` and feeding
the results as inputs to the `<lateral>` pipeline.  Each time the
lateral expression is evaluated, the lateral operators are run to completion,
e.g.,
```mdtest-spq
# spq
values (
  unnest this | sum(this)
)
# input
[3,2,1]
[4,1,7]
[1,2,3]
# expected output
6
12
6
```

This structure generalizes to any more complicated expression context,
e.g., we can embed multiple lateral expressions inside of a record literal
and use the spread operator to tighten up the output:
```mdtest-spq
# spq
{...(unnest this | sort this | sorted:=collect(this)),
 ...(unnest this | sum:=sum(this))}
# input
[3,2,1]
[4,1,7]
[1,2,3]
# expected output
{sorted:[1,2,3],sum:6}
{sorted:[1,4,7],sum:12}
{sorted:[1,2,3],sum:6}
```

Because Zed expressions evaluate to a single result, if multiple values remain
at the conclusion of the lateral pipeline, they are automatically wrapped in
an array, e.g.,
```mdtest-spq
# spq
values {s:(unnest x | values this+1 | collect(this) )}
# input
{x:[2]}
{x:[3,4]}
# expected output
{s:[3]}
{s:[4,5]}
```

To handle such dynamic input data, you can ensure your downstream pipeline
always receives consistently packaged values by explicitly wrapping the result
of the lateral scope, e.g.,
```mdtest-spq
# spq
values {s:(unnest x | values this+1 | collect(this))}
# input
{x:[2]}
{x:[3,4]}
# expected output
{s:[3]}
{s:[4,5]}
```

Similarly, a primitive value may be consistently produced by concluding the
lateral scope with an operator such as [`head`](operators/head.md) or
[`tail`](operators/tail.md), or by applying certain [aggregate functions](aggregates/_index.md)
such as done with [`sum`](aggregates/sum.md) above.
