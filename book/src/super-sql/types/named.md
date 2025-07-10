### Named Types

TODO: move some of this to expressions section

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
1::=foo
2::=bar
3::=foo
# expected output
1::=foo
3::=foo
```

and

```mdtest-spq
# spq
values <foo>
# input
1::=foo
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
1::=foo
2::=bar
"hello"::=foo
3::=foo
# expected output
{typeof:<bar=int64>,count:1::uint64}
{typeof:<foo=int64>,count:2::uint64}
{typeof:<foo=string>,count:1::uint64}
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