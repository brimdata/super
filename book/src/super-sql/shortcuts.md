## Shortcuts

TODO: SuperSQL Shortcuts (change name here from implied operators)

When SuperSQL is utilized in an application like [SuperDB Desktop](https://zui.brimdata.io),
queries are often composed interactively in a "search bar" experience.
The language design here attempts to support both this "lean forward" pattern of usage
along with a "coding style" of query writing where the queries might be large
and complex, e.g., to perform transformations in a data pipeline, where
the SuperSQL queries are stored under source-code control perhaps in GitHub.

To facilitate both a programming-like model as well as an ad hoc search
experience, SuperSQL has a canonical, long form that can be abbreviated
using syntax that supports an agile, interactive query workflow.
To this end, SuperSQL allows certain operator names to be optionally omitted when
they can be inferred from context.  For example, the expression following
the [`aggregate` operator](operators/aggregate.md)
```
aggregate count() by id
```
is unambiguously an aggregation and can be shortened to
```
count() by id
```
Likewise, a very common lean-forward use pattern is "searching", so with the
use of leading `?` shorthand, expressions are interpreted as keyword searches, e.g.,
```
search foo bar or x > 100
```
is abbreviated
```
? foo bar or x > 100
```
Furthermore, if an operator-free expression is not valid syntax for
a search expression but is a valid [expression](expressions.md),
then the abbreviation is treated as having an implied `yield` operator, e.g.,
```
{s:lower(s)}
```
is shorthand for
```
values {s:lower(s)}
```

Another common query pattern involves adding or mutating fields of records
where the input is presumed to be a sequence of records.
The [`put` operator](operators/put.md) provides this mechanism and the `put`
keyword is implied by the [field assignment](#field-assignments) syntax `:=`.

For example, the operation
```
put y:=2*x+1
```
can be expressed simply as
```
y:=2*x+1
```
When composing long-form queries that are shared via SuperDB Desktop or managed in GitHub,
it is best practice to include all operator names in the source text.

In summary, if no operator name is given, the implied operator is determined
from the operator-less source text, in the order given, as follows:
* If the text can be interpreted as a search expression and leading `?` shorthand is used, then the operator is `search`.
* If the text can be interpreted as a boolean expression, then the operator is `where`.
* If the text can be interpreted as one or more field assignments, then the operator is `put`.
* If the text can be interpreted as an aggregation, then the operator is `aggregate`.
* If the text can be interpreted as an expression, then the operator is `yield`.
* Otherwise, the text causes a compile-time error.

When in doubt, you can always check what the compiler is doing under the hood
by running `super` with the `-C` flag to print the parsed query in "canonical form", e.g.,
```mdtest-command
super -C -c '? foo'
super -C -c 'is(<foo>)'
super -C -c 'count()'
super -C -c '{a:x+1,b:y-1}'
super -C -c 'a:=x+1,b:=y-1'
```
produces
```mdtest-output
search foo
where is(<foo>)
aggregate
    count()
values {a:x+1,b:y-1}
put a:=x+1,b:=y-1
```
