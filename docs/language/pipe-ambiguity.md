---
title: The Pipeline Symbol
---

In SuperSQL, you can use one of two symbols to separate pipeline operators: `|` or `|>`.

It's usually better to have just one way of doing things, especially when it comes
to a fundamental element of a query language, so how do we arrive at this decision?

We certainly prefer typing `|` over `|>` especially when interactively exploring data
and quickly editing and running pipeline queries in an iterative fashion.  Having to
perform those awkward keyboard gymnastics from upper right to lower right is just
a pain.

Other query languages use `|` (e.g.,
[Kusto](https://learn.microsoft.com/en-us/kusto/query/?view=microsoft-fabric),
[PRQL](https://prql-lang.org/),
[OxQL](https://rfd.shared.oxide.computer/rfd/0463), and many more)
but Google recently chose `|>` for GoogleSQL pipeline syntax.  Why?

In [their paper on SQL pipes](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/),
Google claims that the `|` character collides with its use in bitwise-OR expressions
to create "parsing ambiguities".  They cite this example as proof:
```sql
FROM Part
| SELECT *, p_size+1 | EXTEND p_type
| SELECT p_name
| AGGREGATE -COUNT(*)
```
arguing that this could be parsed as follows:
```sql
FROM Part
| SELECT *, (p_size+1 | extend) AS p_type
| SELECT (p_name | aggregate) - COUNT(*)
```
But `extend` is a _reserved_ keyword in SQL and `p_size+1 | extend` is
not a valid SQL expression.  This syntax is not intrinsically ambiguous.

The problem they encountered there was that _their_ parser implementation
could not resolve this ambiguity.  LALR parsers like yacc and bison provide for
one token of lookahead to resolve ambiguities yet parsing SQL with `|`
requires additional lookahead: the parser must look past the `|` to
see if it is followed by the start of a new pipeline operator or by more
expression syntax.

So LALR parsers can't handle a SQL that simultaneously uses `|` for pipes
and bitwise-OR.  Sure enough, it appears that GoogleSQL uses an
[LALR parser](https://github.com/google/zetasql/blob/master/bazel/bison.bzl).

SuperSQL on the other hand uses a
[PEG parser](https://en.wikipedia.org/wiki/Parsing_expression_grammar)
and arbitrary lookahead is built into the PEG parsing model so
SuperSQL has no problem using `|` instead of `|>`.
But because GoogleSQL has influence and reach,
and in the spirit of having the familiarity of this emergent pattern, we chose
to support both styles.

That all said, we did bump into a different ambiguity related to the `|` symbol,
namely the use of bitwise-OR followed by one of SuperSQL's
[shortcuts](https://zed.brimdata.io/docs/language/pipeline-model#implied-operators).  For example, this query
```
select 1 | count()
```
has two interpretations independent of parser capabilities.
It's valid first as a bitwise-OR expression, where `count` might be redefined as
a user-defined function, e.g.,
```
select (1 | count())
```
and secondly, as a `select` followed-by a shortcut aggregate, e.g.,
```
(select 1) | count()
```
In other words, SQL pipes _with SuperSQL shortcuts_ is in fact
_intrinsically ambiguous_.

To avoid this problem, SuperSQL takes the suggestion in the Google paper:
when shortcuts are enabled, any top-level bitwise-OR expressions
must be parenthesized.  Because SuperSQL supports
named bitwise functions as a best-practice, there is no need for this
disambiguation when it comes to newly written queries.

When shortcuts aren't enabled, SuperSQL is fully compatible with legacy SQL syntax.
So when you want to run SuperSQL on old SQL queries that use top-level
bitwise-OR expressions --- arguably a pretty obscure corner case --- just disable
SuperSQL shortcuts and everything will work.

{{% tip "Note" %}}

Note that a config option to disable shortcuts is not yet implemented, but will be
available in the future.

{{% /tip %}}
