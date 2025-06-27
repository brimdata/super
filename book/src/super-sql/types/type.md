## Type Type

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
