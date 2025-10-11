## SuperSQL

SuperSQL is a
[Pipe SQL](https://research.google/pubs/sql-has-problems-we-can-fix-them-pipe-syntax-in-sql/)
adapted for
[super-structured data](../formats/model.md).
The language is a superset of SQL query syntax and includes
a modern [type system](types/intro.md) with [sum types](types/union.md) to represent
heterogeneous data.

Similar to a Unix pipeline, a SuperSQL query is expressed as a data source followed
by a number of [operators](operators/intro.md) that manipulate the data:
```
from source | operator | operator | ...
```
As with SQL, SuperSQL is
[declarative](https://en.wikipedia.org/wiki/Declarative_programming)
and the SuperSQL compiler often optimizes a query into an implemention
different from the dataflow implied by the pipeline to achieve the
same semantics with better performance.

### Interactive UX

To support an interactive pattern of usage, SuperSQL includes
[search](operators/search.md) syntax
reminiscent of Web or email keyword search along with
[_syntactic shortcuts_](syntax/shortcuts.md).

With shortcuts, verbose queries can be typed in a shorthand facilitating
rapid data exploration.  For example, the query
```
SELECT count(), key
FROM source
GROUP BY key
```
can be simplified as `from source | count() by key`.

With search, all of the string fields in a value can easily be searched for
patterns, e.g., this query
```
from source
| ? example.com urgent message_length > 100
```
searches for the strings "example.com" and "urgent" in all of the string values in
the input and also includes a numeric comparison regarding the field `message_length`.

### SQL Compatibility

SuperSQL is [backward compatible](../intro.md#supersql)
with relational SQL in that any SQL query is also a SuperSQL query.
An arbitrarily complex SQL query may appear as a single [pipe operator](operators/intro.md)
anywhere in a SuperSQL pipe query.

In other words, a single pipe operator that happens to be a SQL query
is also a SuperSQL pipe query.
For example, these are all valid SuperSQL queries:
```
SELECT 'hello, world'
SELECT * FROM table
SELECT * FROM f1.json JOIN f2.json ON f1.id=f2.id
SELECT watchers FROM https://api.github.com/repos/brimdata/super
```

### Pipe Queries

The entities that transform data within a SuperSQL pipeline are called
[pipe operators](operators/intro.md)
and take super-structured input from the upstream operator or data source,
operate upon the input, and produce zero or more super-structured
values as output.

Unlike relational SQL, SuperSQL pipe queries define their computation in terms of dataflow
through the directed graph of operators.  But instead of relational tables
propagating from one pipe operator to another
(e.g., as in [Zeta pipe SQL](https://github.com/google/zetasql/blob/master/docs/pipe-syntax.md#pipe-operator-semantics)), any sequence of potentially heterogeneously typed data
may flow between SuperSQL pipe operators.

When a super-structured sequence conforms to a single, homoegeneous
[record type](types/record.md),
then the data is equivalent to a SQL relation.
And because [any SQL query is also a valid pipe operator](sql/intro.md),
SuperSQL is thus a superset of SQL.
In particular, a single operator defined as pure SQL is an
acceptable SuperSQL query so all SQL query texts are also SuperSQL queries.

Unlike a Unix pipeline, a SuperSQL query can be forked and joined, e.g.,
```
from source
| operator
| fork
  ( operator | ... )
  ( operator | ... )
| join on condition
| ...
| switch expr
  case value ( operator | ... )
  case value ( operator | ... )
  default ( operator | ... )
| ...
```
A query can also include multiple data sources, e.g.,
```
fork
  ( from source1 | ... )
  ( from source2 | ... )
| ...
```

### Pipe Sources

Like SQL, input data for a query is typically sourced with the
[`from`](operators/from.md) operator.

When [`from`](operators/from.md) is not present, the file arguments to the
[`super`](../commands/super.md) command are used as input to the query
as if there is an implied
[`from`](operators/from.md) operator, e.g.,
```
super -c "op1 | op2 | ..." input.json
```
is equivalent to
```
super -c "from input.json | op1 | op2 | ..."
```
When neither
[`from`](operators/from.md) nor file arguments are specified, a single `null` value
is provided as input to the query.
```
super -c "pass"
```
results in
```
null
```
This pattern provides a simple means to produce a constant input within a
query using the [`values`](operators/values.md) operator, wherein
[`values`](operators/values.md) takes as input a single null and produces each constant
expression in turn, e.g.,
```
super -c "values 1,2,3"
```
results in
```
1
2
3
```

When running on the local file system,
[`from`](operators/from.md) may refer to a file or an HTTP URL
indicating an API endpoint.
When connected to [SuperDB database](../database/intro.md),
[`from`](operators/from.md) typically
refers to a collection of super-structured data called a _pool_ and
is referenced using the pool's name similar to SQL referencing
a relational table by name.

For more detail, see the reference page of the [`from`](operators/from.md) operator,
but as an example, you might use its
[`from`](operators/from.md) to fetch data from an
HTTP endpoint and process it with `super`, in this case, to extract the description
and license of a GitHub repository:
```
super -f line -c "
from https://api.github.com/repos/brimdata/super
| values description,license.name
"
```

### Relational Scoping

In SQL queries, data from tables is generally referenced with expressions that
specify a table name and a column name within that table,
e.g., referencing a column `x` in a table `this` as
```
SELECT this.x FROM (VALUES (1),(2),(3)) AS this(x)
```
More commonly, when the column name is unambiguous, the table name
can be omitted as in
```
SELECT x FROM (VALUES (1),(2),(3)) AS this(x)
```
When SQL queries are nested, joined, or invoked as subqueries, scoping
rules define how identifiers and dotted expressions resolve to the
different available table names and columns reachable via containing scopes.
To support such semantics, SuperSQL implements SQL scoping rules
_inside of of any SQL pipe operator_ but not between pipe operators.

In other words, table aliases and column references all work within
a SQL query written as a single pipe operator but scoping of tables
and columns does not reach across pipe operators.  Likewise, a pipe query
embedded inside of a nested SQL query cannot access tables and columns in
the containing SQL scope.

### Dataflow Scoping

For pipe queries, SuperSQL takes a different approach to scoping
called _dataflow scoping_.

Here, a pipe operator takes any sequence of input values
and produces any computed sequence of output values and _all
data references are limited to these inputs and outputs_.
Since there is just one sequence of values, it may be
referenced as special value with a special name, which for
SuperSQL is the value `this`.

This scoping model can be summarized as follows:
* all input is referenced as a single value called `this`, and
* all output is emitted into a single value called `this`.

As mentioned above,
when processing a set of homogeneously-typed [records](types/record.md),
the data resembles a relational table where the record type resembles a
relational schema and each field in the record models the table's column.
In other words, the record fields of `this` can be accessed with the dot operator
reminiscent of a `table.column` reference in SQL.

For example. ,the SQL query from above can thus be written in pipe form
using the [`values`](operators/values.md) operator as:
```
values {x:1}, {x:2}, {x:3} | select this.x
```
which results in:
```
{x:1}
{x:2}
{x:3}
```
As with SQL table names, where `this` is implied, it is optional can be omitted, i.e.,
```
values {x:1}, {x:2}, {x:3} | select x
```
produces the same result.

Referencing `this` is often convenient, however, as in this query
```
values {x:1}, {x:2}, {x:3} | aggregate collect(this)
```
which collects each input value into an array and emits the array resulting in
```
[{x:1},{x:2},{x:3}]
```

#### Combining Piped Data

If all data for all operators were always presented as a single input sequence
called `this`, then there would be no way to combine data from different entities,
which is otherwise a hallmark of SQL and the relational model.

To remedy this, SuperSQL extends dataflow scoping to
[_joins_](#join-scoping) and
[_subqueries_](#subquery-scoping)
where multiple entities can be combined into the common value `this`.

##### Join Scoping

To combine joined entities into `this` via dataflow scoping, the
[`join`](operators/join.md) operator
includes an _as clause_ that names the two sides of the join, e.g.,
```
... | join ( from ... ) as {left,right} | ...
```
Here, the joined values are formed into a new two-field record
whose first field is `left` and whose second field is `right` where the
`left` values come from the parent operator and the `right` values come
from the parenthesized join query argument.

For example, suppose the contents of a file `f1.json` is
```
{"x":1}
{"x":2}
{"x":3}
```
and `f2.json` is
```
{"y":4}
{"y":5}
```
then a `join` can bring these two entities together into a common record
which can then be subsequently operated upon, e.g.,
```
from f1.json
| cross join (from f2.json) as {f1,f2}
```
computes a cross-product over all the two sides of the join
and produces the following output
```
{f1:{x:1},f2:{y:4}}
{f1:{x:2},f2:{y:4}}
{f1:{x:3},f2:{y:4}}
{f1:{x:1},f2:{y:5}}
{f1:{x:2},f2:{y:5}}
{f1:{x:3},f2:{y:5}}
```
A downstream operator can then operate on these records,
for example, merging the two sides of the join using
spread operators (`...`), i.e.,
```
from f1.json
| cross join (from f2.json) as {f1,f2}
| values {...f1,...f2}
```
produces
```
{x:1,y:4}
{x:2,y:4}
{x:3,y:4}
{x:1,y:5}
{x:2,y:5}
{x:3,y:5}
```
In contrast, relational scoping using identifer scoping in a `SELECT` clause
with the table source identified in `FROM` and `JOIN` clauses, e.g., this query
produces the same result:
```
SELECT f1.x, f2.y FROM f1.json as f1 CROSS JOIN f2.json as f2
```

##### Subquery Scoping

A subquery embedded in an expression can also combine data entities
via dataflow scoping as in
```
from outer | values {outer:this,inner:(from inner | ...)}
```
Here data from the outer query can be mixed in with data from the
inner query embedded in the expression inside of the
[`values`](operators/values.md) operator.

The subquery produces an array value so it is often desirable to
[`unnest`](operators/unnest.md) this array with respect to the outer
values as in
```
from f1.json | unnest {outer:this,inner:(from f2.json | ...)} into ( <scope> )
```
where `<scope>` can be an arbitrary pipe query that processes each
collection of unnested values separately as a unit for each outer value.
The `into ( <scope> )` body is an optional component of `unnest`, and if absent,
the unnested collection boundaries are ignored and all of the unnested data is output.

With the `unnest` operator, we can now consider how a correlated subquery from
SQL can be implemented purely as a pipe query with dataflow scoping.
For example,
```
SELECT (SELECT sum(f1.x+f2.y) FROM f1.json) AS s FROM f2.json
```
results in
```
{s:18}
{s:21}
```
To implement this with dataflow scoping,
the correlated subquery is carried out by
unnesting the data from the subquery with the values coming from the outer
scope as in
```
from f2.json
| unnest {f2:this,f1:(from f1.json)} into ( s:=sum(f1.x+f2.y) )
```
giving the same result
```
{s:18}
{s:21}
```

### Strong Typing

Data in SuperSQL is always strongly typed.

Like retional SQL, SuperSQL data sequences
can conform to a static schema that is type-checked at compile time.
And like
[document databases](https://en.wikipedia.org/wiki/Document-oriented_database)
and [SQL++](https://asterixdb.apache.org/files/SQL_Book.pdf),
data sequences may also be dynamically typed, but unlike these systems,
SuperSQL data is always strongly typed.

For example, this query produces the expected output
```
$ super -c "select b from (values (1,2),(3,4)) as T(b,c)"
{b:1}
{b:2}
```
But this query produced a compile-time error:
```
$ super -c "select a from (values (1,2),(3,4)) as T(b,c)"
column "a": does not exist at line 1, column 8:
select a from (values (1,2),(3,4)) as T(b,c)
       ~
```
Now supposing this data is in the file `input.json`:
```
{"b":1,"c":2}
{"b":3,"c":4}
```
Then, the query from above would run but because there is no predefined
schema for the sequence of JSON, "missing" values are produced:
```
$ super -c "select a from input.json"
{a:error("missing")}
{a:error("missing")}
```
While this data is strongly typed, it is also dynamically typed so that
the types are not necessarily all known ahead of time, which sometimes precludes
compile-time type checking.

That is, even though the reference to column "a" is dynamically evaluated, all
the data is still strongly typed, i.e.,
 ```
$ super -c "from input.json | values typeof(this)"
<{b:int64,c:int64}>
<{b:int64,c:int64}>
```
> While SuperSQL currently uses aschema information to check the existence of
> column references, type checking using the actual column types is
> not yet performed at compile time and is instead detected dynamically at run time.
> For example, `select "foo"+1 as x` produces the runtime value `{x:error("missing")}`.
> Future versions of SuperSQL will do comprehensive type checking and report such
> errors at compile time.  Similarly, future versions will use the type information
> of super-structured file formats to perform compile-time type checking for
> heterogeous data inputs.

### Data Order

Data sequences from sources may have a natural order.  For example,
the values in a file being read are presumed to have the order they
appear inthe file.  Likewise, data stored in a database organized by
a sort constraint is presumed to have the sorted order.

For _order-preserving_ pipe operators, this order is preserved.
For _order-creating_ operators like [`sort`](operators/sort.md)
an output order is created independent of the input order.
For other operators, the output order is undefined.

Each operator defines whether or not it is order is preserved,
created, or discarded.

For example, the [`where`](operators/where.md) drops values that do
not meet the operator's condition but otherwise preserves data order,
whereas the [`sort`](operators/sort.md) creates an output order defined
by the sort expressions.  The [`aggregate`](operators/aggregate.md)
creates an undefined order at output.

When a pipe query branches as in
[`join`](operators/join.md),
[`fork`](operators/fork.md), or
[`switch`](operators/switch.md),
the order at the merged branches is undefined.
