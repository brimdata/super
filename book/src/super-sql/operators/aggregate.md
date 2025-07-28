### Operator

&emsp; **aggregate** &mdash; execute aggregation functions with optional group-by expressions

### Synopsis

```
[aggregate] [<field>:=]<agg>
[aggregate] [<field>:=]<agg> [where <expr>][, [<field>:=]<agg> [where <expr>] ...]
[aggregate] [<field>:=]<agg> [by [<field>][:=<expr>][, [<field>][:=<expr>]] ...]
[aggregate] [<field>:=]<agg> [where <expr>][, [<field>:=]<agg> [where <expr>] ...] [by [<field>][:=<expr>][, [<field>][:=<expr>]] ...]
[aggregate] by [<field>][:=<expr>][, [<field>][:=<expr>] ...]
```

### Description

The `aggregate` operator  aggregates groups of its input to reduce
each group of values to one or more values  according to one or more
[aggregate functions](../aggregates/intro.md)
When there is no grouping clause, the aggregate functions are applied to the entire input.

In the first four forms, the `aggregate` operator consumes all of its input,
applies one or more aggregate functions `<agg>` to each input value
optionally filtered by a `where` clause and/or organized with the grouping
keys specified after the `by` keyword, and at the end of input produces one
or more aggregations for each unique set of grouping key values.

In the final form, `aggregate` consumes all of its input, then outputs each
unique combination of values of the grouping keys specified after the `by`
keyword.

The `aggregate` keyword is optional since it can be used as a
[shortcut](../shortcuts.md).

Each aggregate function may be optionally followed by a `where` clause, which
applies a Boolean expression `<expr>` that indicates, for each input value,
whether to deliver it to that aggregate. `where` clauses are analogous
to the [`where`](where.md) operator but apply their filter to the input
argument stream to the aggregatge function.

The output field names for each aggregate and each key are optional.  If omitted,
a field name is inferred from each right-hand side, e.g., the output field for the
[`count`](../aggregates/count.md) aggregate function is simply `count`.

A key may be either an expression or a field.  If the key field is omitted,
it is inferred from the expression, e.g., the field name for `by lower(s)`
is `lower`.

When the result of `aggregate` is a single value (e.g., a single aggregate
function without grouping keys) and there is no field name specified, then
the output is that single value rather than a single-field record
containing that value.

If the cardinality of grouping keys causes the memory footprint to exceed
a limit, then each aggregate's partial results are spilled to temporary storage
and the results merged into final results using an external merge sort.

> Spilling is not yet implemented for the vectorized runtime.

### Examples

---

Average the input sequence:
```mdtest-spq
# spq
aggregate avg(this)
# input
1
2
3
4
# expected output
2.5
```

---

To format the output of a single-valued aggregation into a record, simply specify
an explicit field for the output:
```mdtest-spq
# spq
aggregate mean:=avg(this)
# input
1
2
3
4
# expected output
{mean:2.5}
```

---

When multiple aggregate functions are specified, even without explicit field names,
a record result is generated with field names implied by the functions:
```mdtest-spq
# spq
aggregate avg(this),sum(this),count()
# input
1
2
3
4
# expected output
{avg:2.5,sum:10,count:4::uint64}
```

---

Sum the input sequence, leaving out the `aggregate` keyword:
```mdtest-spq
# spq
sum(this)
# input
1
2
3
4
# expected output
10
```

---

Create integer sets by key and sort the output to get a deterministic order:
```mdtest-spq
# spq
set:=union(v) by key:=k | sort
# input
{k:"foo",v:1}
{k:"bar",v:2}
{k:"foo",v:3}
{k:"baz",v:4}
# expected output
{key:"bar",set:|[2]|}
{key:"baz",set:|[4]|}
{key:"foo",set:|[1,3]|}
```

---

Use a `where` clause:
```mdtest-spq
# spq
set:=union(v) where v > 1 by key:=k | sort
# input
{k:"foo",v:1}
{k:"bar",v:2}
{k:"foo",v:3}
{k:"baz",v:4}
# expected output
{key:"bar",set:|[2]|}
{key:"baz",set:|[4]|}
{key:"foo",set:|[3]|}
```

---

Use separate `where` clauses on each aggregate function:
```mdtest-spq
# spq
set:=union(v) where v > 1,
array:=collect(v) where k=="foo"
  by key:=k
| sort
# input
{k:"foo",v:1}
{k:"bar",v:2}
{k:"foo",v:3}
{k:"baz",v:4}
# expected output
{key:"bar",set:|[2]|,array:null}
{key:"baz",set:|[4]|,array:null}
{key:"foo",set:|[3]|,array:[1,3]}
```

---

Results are included for `by` groupings that generate null results when `where`
clauses are used inside `aggregate`:
```mdtest-spq
# spq
sum(v) where k=="bar" by key:=k | sort
# input
{k:"foo",v:1}
{k:"bar",v:2}
{k:"foo",v:3}
{k:"baz",v:4}
# expected output
{key:"bar",sum:2}
{key:"baz",sum:null}
{key:"foo",sum:null}
```

---

To avoid null results for `by` groupings as just shown, filter before `aggregate`:
```mdtest-spq
# spq
k=="bar" | sum(v) by key:=k | sort
# input
{k:"foo",v:1}
{k:"bar",v:2}
{k:"foo",v:3}
{k:"baz",v:4}
# expected output
{key:"bar",sum:2}
```

---

Output just the unique key values:
```mdtest-spq
# spq
by k | sort
# input
{k:"foo",v:1}
{k:"bar",v:2}
{k:"foo",v:3}
{k:"baz",v:4}
# expected output
{k:"bar"}
{k:"baz"}
{k:"foo"}
```
