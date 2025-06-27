---
weight: 3
title: Data Types
---

The SuperSQL language includes most data types of a typical programming language
as defined in the [super data model](../formats/data-model.md).

The syntax of individual literal values generally follows
the [Super (SUP) syntax](../formats/sup.md) with the exception that
[type decorators](../formats/sup.md#22-type-decorators)
are not included in the language.  Instead, a
[type cast](expressions.md#casts) may be used in any expression for explicit
type conversion.

In particular, the syntax of primitive types follows the
[primitive-value definitions](../formats/sup.md#23-primitive-values) in SUP
as well as the various [complex value definitions](../formats/sup.md#24-complex-values)
like records, arrays, sets, and so forth.  However, complex values are not limited to
constant values like SUP and can be composed from [literal expressions](expressions.md#literals).

## First-class Types

As in the super data model, the SuperSQL language has first-class types:
any type may be used as a value.

The primitive types are listed in the
[data model specification](../formats/data-model.md#1-primitive-types)
and have the same syntax in SuperSQL.  Complex types also follow
the SUP syntax.  Note that the type of a type value is simply `type`.

As in SUP, _when types are used as values_, e.g., in an expression,
they must be referenced within angle brackets.  That is, the integer type
`int64` is expressed as a type value using the syntax `<int64>`.

Complex types in the SuperSQL language follow the SUP syntax as well.  Here are
a few examples:
* a simple record type - `{x:int64,y:int64}`
* an array of integers - `[int64]`
* a set of strings - `|[string]|`
* a map of strings keys to integer values - `{[string,int64]}`
* a union of string and integer  - `(string,int64)`

Complex types may be composed, as in `[({s:string},{x:int64})]` which is
an array of type `union` of two types of records.

The [`typeof` function](functions/typeof.md) returns a value's type as
a value, e.g., `typeof(1)` is `<int64>` and `typeof(<int64>)` is `<type>`.

First-class types are quite powerful because types can
serve as grouping keys or be used in ["data shaping"](shaping.md) logic.
A common workflow for data introspection is to first perform a search of
exploratory data and then count the shapes of each type of data as follows:
```
search ... | count() by typeof(this)
```
For example,
``` mdtest-spq
# spq
count() by typeof(this) | sort this
# input
1
2
"foo"
10.0.0.1
<string>
# expected output
{typeof:<int64>,count:2(uint64)}
{typeof:<string>,count:1(uint64)}
{typeof:<ip>,count:1(uint64)}
{typeof:<type>,count:1(uint64)}
```

When running such a query over complex, semi-structured data, the results can
be quite illuminating and can inform the design of "data shaping" queries
to transform raw, messy data into clean data for downstream tooling.

Note the somewhat subtle difference between a record value with a field `t` of
type `type` whose value is type `string`
```
{t:<string>}
```
and a record type used as a value
```
<{t:string}>
```

## Named Types

As in any modern programming language, types can be named and the type names
persist into the data model and thus into the serialized input and output.

Named types may be defined in four ways:
* with a [`type` statement](statements.md#type-statements),
* with the [`cast` function](functions/cast.md),
* with a definition inside of another type, or
* by the input data itself.

Type names that are embedded in another type have the form
```
name=type
```
and create a binding between the indicated string `name` and the specified type.
For example,
```
type socket = {addr:ip,port:port=uint16}
```
defines a named type `socket` that is a record with field `addr` of type `ip`
and field `port` of type "port", where type "port" is a named type for type `uint16` .

Named types may also be defined by the input data itself, as super-structured data is
comprehensively self describing.
When named types are defined in the input data, there is no need to declare their
type in a query.
In this case, a SuperSQL expression may refer to the type by the name that simply
appears to the runtime as a side effect of operating upon the data, e.g.,

```mdtest-spq
# spq
typeof(this)==<foo>
# input
1(=foo)
2(=bar)
3(=foo)
# expected output
1(=foo)
3(=foo)
```

and

```mdtest-spq
# spq
values <foo>
# input
1(=foo)
# expected output
<foo=int64>
```

If the type name referred to in this way does not exist, then the type value
reference results in `error("missing")`.  For example,
```mdtest-spq
# spq
values <foo>
# input
1
# expected output
error("missing")
```

Each instance of a named type definition overrides any earlier definition.
In this way, types are local in scope.

Each value that references a named type retains its local definition of the
named type retaining the proper type binding while accommodating changes in a
particular named type.  For example,
```mdtest-spq {data-layout="stacked"}
# spq
count() by typeof(this) | sort this
# input
1(=foo)
2(=bar)
"hello"(=foo)
3(=foo)
# expected output
{typeof:<bar=int64>,count:1(uint64)}
{typeof:<foo=int64>,count:2(uint64)}
{typeof:<foo=string>,count:1(uint64)}
```

Here, the two versions of type "foo" were retained in the aggregation results.

In general, it is bad practice to define multiple versions of a single named type,
though the SuperDB system and super data model accommodate such dynamic bindings.
Managing and enforcing the relationship between type names and their type definitions
on a global basis (e.g., across many different data pools in a data lake) is outside
the scope of the super data model and SuperSQL language.  That said, SuperDB provides flexible
building blocks so systems can define their own schema versioning and schema
management policies on top of these primitives.

The [super-structured data model](../formats/_index.md#2-a-super-structured-pattern)
is a superset of relational tables and
SuperSQL's type system can easily make this connection.
As an example, consider this type definition for "employee":
```
type employee = {id:int64,first:string,last:string,job:string,salary:float64}
```
In SQL, you might find the top five salaries by last name with
```
SELECT last,salary
FROM employee
ORDER BY salary
LIMIT 5
```
Using pipes in SuperSQL, you could say
```
from anywhere
| typeof(this)==<employee>
| cut last,salary
| sort salary
| head 5
```
and since type comparisons are so useful and common, the [`is` function](functions/is.md)
can be used to perform the type match:
```
from anywhere
| is(<employee>)
| cut last,salary
| sort salary
| head 5
```
The power of SuperSQL is that you can interpret data on the fly as belonging to
a certain schema, in this case "employee", and those records can be intermixed
with other relevant data.  There is no need to create a table called "employee"
and put the data into the table before that data can be queried as an "employee".
And if the schema or type name for "employee" changes, queries still continue
to work.

## First-class Errors

As with types, errors in SuperSQL are first-class: any value can be transformed
into an error by wrapping it in an [`error` type](../formats/data-model.md#27-error).

In general, expressions and functions that result in errors simply return
a value of type `error` as a result.  This encourages a powerful flow-style
of error handling where errors simply propagate from one operation to the
next and land in the output alongside non-error values to provide a very helpful
context and rich information for tracking down the source of errors.  There is
no need to check for error conditions everywhere or look through auxiliary
logs to find out what happened.

For example,
input values can be transformed to errors as follows:
```mdtest-spq
# spq
error(this)
# input
0
"foo"
10.0.0.1
# expected output
error(0)
error("foo")
error(10.0.0.1)
```

More practically, errors from the runtime show up as error values.
For example,
```mdtest-spq
# spq
1/this
# input
0
# expected output
error("divide by zero")
```

And since errors are first-class and just values, they have a type.
In particular, they are a complex type where the error value's type is the
complex type `error` containing the type of the value.  For example,
```mdtest-spq
# spq
typeof(1/this)
# input
0
# expected output
<error(string)>
```

First-class errors are particularly useful for creating structured errors.
When a SuperSQL query encounters a problematic condition,
instead of silently dropping the problematic error
and logging an error obscurely into some hard-to-find system log as so many
ETL pipelines do, the offending value can be wrapped as an error and
propagated to its output.

For example, suppose a bad value shows up:
```
{kind:"bad", stuff:{foo:1,bar:2}}
```
A [shaper](shaping.md) could catch the bad value (e.g., as a default
case in a [`switch`](operators/switch.md) topology) and propagate it as
an error using the expression:
```
values error({message:"unrecognized input",input:this})
```
then such errors could be detected and searched for downstream with the
[`is_error` function](functions/is_error.md).
For example,
```
is_error(this)
```
on the wrapped error from above produces
```
error({message:"unrecognized input",input:{kind:"bad", stuff:{foo:1,bar:2}}})
```
There is no need to create special tables in a complex warehouse-style ETL
to land such errors as they can simply land next to the output values themselves.

And when transformations cascade one into the next as different stages of
an ETL pipeline, errors can be wrapped one by one forming a "stack trace"
or lineage of where the error started and what stages it traversed before
landing at the final output stage.

Errors will unfortunately and inevitably occur even in production,
but having a first-class data type to manage them all while allowing them to
peacefully coexist with valid production data is a novel and
useful approach that SuperSQL enables.

### Missing and Quiet

SuperDB's heterogeneous data model allows for queries
that operate over different types of data whose structure and type
may not be known ahead of time, e.g., different
types of records with different field names and varying structure.
Thus, a reference to a field, e.g., `this.x` may be valid for some values
that include a field called `x` but not valid for those that do not.

What is the value of `x` when the field `x` does not exist?

A similar question faced SQL when it was adapted in various different forms
to operate on semi-structured data like JSON or XML.  SQL already had the `NULL` value
so perhaps a reference to a missing value could simply be `NULL`.

But JSON also has `null`, so a reference to `x` in the JSON value
```
{"x":null}
```
and a reference to `x` in the JSON value
```
{}
```
would have the same value of `NULL`.  Furthermore, an expression like `x==NULL`
could not differentiate between these two cases.

To solve this problem, the `MISSING` value was proposed to represent the value that
results from accessing a field that is not present.  Thus, `x==NULL` and
`x==MISSING` could disambiguate the two cases above.

SuperSQL, instead, recognizes that the SQL value `MISSING` is a paradox:
I'm here but I'm not.

In reality, a `MISSING` value is not a value.  It's an error condition
that resulted from trying to reference something that didn't exist.

So why should we pretend that this is a bona fide value?  SQL adopted this
approach because it lacks first-class errors.

But SuperSQL has first-class errors so
a reference to something that does not exist is an error of type
`error(string)` whose value is `error("missing")`.  For example,
```mdtest-spq
# spq
values x
# input
{x:1}
{y:2}
# expected output
1
error("missing")
```

Sometimes you want missing errors to show up and sometimes you don't.
The [`quiet` function](functions/quiet.md) transforms missing errors into
"quiet errors".  A quiet error is the value `error("quiet")` and is ignored
by most operators, in particular `values`.  For example,
```mdtest-spq
# spq
values quiet(x)
# input
{x:1}
{y:2}
# expected output
1
```

And what if you want a default value instead of a "missing" error?  The
[`coalesce` function](functions/coalesce.md) returns the first value that is not
null, `error("missing")`, or `error("quiet")`.  For example,
```mdtest-spq
# spq
values coalesce(x, 0)
# input
{x:1}
{y:2}
# expected output
1
0
```
