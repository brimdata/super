---
weight: 5
title: Expressions
---

Expressions follow the typical patterns in programming languages.
Expressions are often used within [operators](operators/_index.md)
to perform computations on input values and are typically evaluated once per each
input value [`this`](pipeline-model.md#the-special-value-this).

For example, [`yield`](operators/yield.md), [`where`](operators/where.md),
[`cut`](operators/cut.md), [`put`](operators/put.md),
[`sort`](operators/sort.md) and so forth all take
various expressions as part of their operation.

## Arithmetic

Arithmetic operations (`*`, `/`, `%`, `+`, `-`) follow customary syntax
and semantics and are left-associative with multiplication and division having
precedence over addition and subtraction.  `%` is the modulo operator.

For example,
```mdtest-spq
# spq
yield 2*3+1, 11%5, 1/0, "foo"+"bar"
# input
null
# expected output
7
1
error("divide by zero")
"foobar"
```

## Comparisons

Comparison operations (`<`, `<=`, `==`, `!=`, `>`, `>=`) follow customary syntax
and semantics and result in a truth value of type `bool` or an [error](data-types.md#first-class-errors).
A comparison expression is any valid expression compared to any other
valid expression using a comparison operator.

When the operands are coercible to like types, the result is the truth value
of the comparison.  Otherwise, the result is `false`.

If either operand to a comparison
is `error("missing")`, then the result is `error("missing")`.

For example,
```mdtest-spq
# spq
yield 1 > 2, 1 < 2, "b" > "a", 1 > "a", 1 > x
# input
null
# expected output
false
true
true
false
error("missing")
```

## Containment

The `in` operator has the form
```
<item-expr> in <container-expr>
```
and is true if the `<item-expr>` expression results in a value that
appears somewhere in the `<container-expr>` as an exact match of the item.
The right-hand side value can be any value. For example,
```mdtest-spq
# spq
1 in this
# input
{a:[1,2]}
{b:{c:3}}
{d:{e:1}}
# expected output
{a:[1,2]}
{d:{e:1}}
```

Complex values are recursively traversed to determine if the item is present
anywhere within them:
```mdtest-spq
# spq
{s:"foo"} in this
# input
{s:"foo"}
{s:"foo",t:"bar"}
{a:{s:"foo"}}
[1,{s:"foo"},2]
# expected output
{s:"foo"}
{a:{s:"foo"}}
[1,{s:"foo"},2]
```

You can also use this operator with a static array:
```mdtest-spq
# spq
over accounts | where id in [1,2]
# input
{accounts:[{id:1},{id:2},{id:3}]}
# expected output
{id:1}
{id:2}
```

## Logic

The keywords `and`, `or`, `not`, and `!` perform logic on operands of type `bool`.
The binary operators `and` and `or` operate on Boolean values and result in
an error value if either operand is not a Boolean.  Likewise, `not` (and its
equivalent `!`) operates on its unary operand and results in an error if its
operand is not type `bool`. Unlike many other languages, non-Boolean values are
not automatically converted to Boolean type using "truthiness" heuristics.

## Field Dereference

Record fields are dereferenced with the dot operator `.` as is customary
in other languages and have the form
```
<value> . <id>
```
where `<id>` is an identifier representing the field name referenced.
If a field name is not representable as an identifier, then [indexing](#indexing)
may be used with a quoted string to represent any valid field name.
Such field names can be accessed using
[`this`](pipeline-model.md#the-special-value-this) and an array-style reference, e.g.,
`this["field with spaces"]`.

If the dot operator is applied to a value that is not a record
or if the record does not have the given field, then the result is
`error("missing")`.

## Indexing

The index operation can be applied to various data types and has the form:
```
<value> [ <index> ]
```
If the `<value>` expression is a record, then the `<index>` operand
must be coercible to a string and the result is the record's field
of that name.

If the `<value>` expression is an array, then the `<index>` operand
must be coercible to an integer and the result is the
value in the array of that index.

If the `<value>` expression is a set, then the `<index>` operand
must be coercible to an integer and the result is the
value in the set of that index ordered by total order of values.

If the `<value>` expression is a map, then the `<index>` operand
is presumed to be a key and the corresponding value for that key is
the result of the operation.  If no such key exists in the map, then
the result is `error("missing")`.

If the `<value>` expression is a string, then the `<index>` operand
must be coercible to an integer and the result is an integer representing
the unicode code point at that offset in the string.

If the `<value>` expression is type `bytes`, then the `<index>` operand
must be coercible to an integer and the result is an unsigned 8-bit integer
representing the byte value at that offset in the bytes sequence.

## Slices

The slice operation can be applied to various data types and has the form:
```
<value> [ <from> : <to> ]
```
The `<from>` and `<to>` terms must be expressions that are coercible
to integers and represent a range of index values to form a subset of elements
from the `<value>` term provided.  The range begins at the `<from>` position
and ends one before the `<to>` position.  A negative
value of `<from>` or `<to>` represents a position relative to the
end of the value being sliced.

If the `<value>` expression is an array, then the result is an array of
elements comprising the indicated range.

If the `<value>` expression is a set, then the result is a set of
elements comprising the indicated range ordered by total order of values.

If the `<value>` expression is a string, then the result is a substring
consisting of unicode code points comprising the given range.

If the `<value>` expression is type `bytes`, then the result is a bytes sequence
consisting of bytes comprising the given range.

## Conditional

A conditional expression has the form
```
<boolean> ? <expr> : <expr>
```
The `<boolean>` expression is evaluated and must have a result of type `bool`.
If not, an error results.

If the result is true, then the first `<expr>` expression is evaluated and becomes
the result.  Otherwise, the second `<expr>` expression is evaluated and
becomes the result.

For example,
```mdtest-spq
# spq
yield (s=="foo") ? v : -v
# input
{s:"foo",v:1}
{s:"bar",v:2}
# expected output
1
-2
```

Conditional expressions can be chained, providing behavior equivalent to
"else if" as appears in other languages.

For example,
```mdtest-spq
# spq
yield (s=="foo") ? v : (s=="bar") ? -v : v*v
# input
{s:"foo",v:1}
{s:"bar",v:2}
{s:"baz",v:3}
# expected output
1
-2
9
```

Note that if the expression has side effects,
as with [aggregate function calls](expressions.md#aggregate-function-calls), only the selected expression
will be evaluated.

For example,
```mdtest-spq
# spq
yield this=="foo" ? {foocount:count()} : {barcount:count()}
# input
"foo"
"bar"
"foo"
# expected output
{foocount:1(uint64)}
{barcount:1(uint64)}
{foocount:2(uint64)}
```

## Function Calls

Functions perform stateless transformations of their input value to their return
value and utilize call-by value semantics with positional and unnamed arguments.

For example,
```mdtest-spq
# spq
yield pow(2,3), lower("ABC")+upper("def"), typeof(1)
# input
null
# expected output
8.
"abcDEF"
<int64>
```

Some [built-in functions](functions/_index.md) take a variable number of
arguments.

[User-defined functions](statements.md#func-statements) may also be created.

## Aggregate Function Calls

[Aggregate functions](aggregates/_index.md) may be called within an expression.
Unlike the aggregation context provided by the [`aggregate` operator](operators/aggregate.md),
such calls in expression context yield an output value for each input value.

Note that because aggregate functions carry state which is typically
dependent on the order of input values, their use can prevent the runtime
optimizer from parallelizing a query.

That said, aggregate function calls can be quite useful in a number of contexts.
For example, a unique ID can be assigned to the input quite easily:
```mdtest-spq
# spq
yield {id:count(),value:this}
# input
"foo"
"bar"
"baz"
# expected output
{id:1(uint64),value:"foo"}
{id:2(uint64),value:"bar"}
{id:3(uint64),value:"baz"}
```

In contrast, calling aggregate functions within the [`aggregate` operator](operators/aggregate.md)
produces just one output value.
```mdtest-spq {data-layout="stacked"}
# spq
aggregate count(),union(this)
# input
"foo"
"bar"
"baz"
# expected output
{count:3(uint64),union:|["bar","baz","foo"]|}
```

## Literals

Any of the [data types](data-types.md) may be used in expressions
as long as it is compatible with the semantics of the expression.

String literals are enclosed in either single quotes or double quotes and
must conform to UTF-8 encoding and follow the JavaScript escaping
conventions and unicode escape syntax.

### Formatted String Literals

A formatted string literal (or f-string) is a string literal prefixed with `f`.
These strings may include replacement expressions which are delimited by curly
braces:
```
f"{ <expr> }"
```
In this case, the characters starting with `{` and ending at `}` are substituted
with the result of evaluating the expression `<expr>`.  If this result is not
a string, it is implicitly cast to a string.

For example,
```mdtest-spq {data-layout="stacked"}
# spq
yield f"pi is approximately {numerator / denominator}"
# input
{numerator:22.0, denominator:7.0}
# expected output
"pi is approximately 3.142857142857143"
```

If any expression results in an error, then the value of the f-string is the
first error encountered in left-to-right order.

F-strings may be nested, where a child `<expr>` may contain f-strings.

For example,
```mdtest-spq {data-layout="stacked"}
# spq
yield f"oh {this[upper(f"{foo + bar}")]}"
# input
{foo:"hello", bar:"world", HELLOWORLD:"hi!"}
# expected output
"oh hi!"
```

To represent a literal `{` character inside an f-string, it must be escaped,
i.e., `\{`.

For example,
```mdtest-spq
# spq
yield f"{this} look like: \{ }"
# input
"brackets"
# expected output
"brackets look like: { }"
```

### Record Expressions

Record literals have the form
```
{ <spec>, <spec>, ... }
```
where a `<spec>` has one of three forms:
```
<field> : <expr>
<ref>
...<expr>
```
The first form is a customary colon-separated field and value similar to JavaScript,
where `<field>` may be an identifier or quoted string.
The second form is an [implied field reference](pipeline-model.md#implied-field-references)
`<ref>`, which is shorthand for `<ref>:<ref>`.  The third form is the `...`
spread operator which expects a record value as the result of `<expr>` and
inserts all of the fields from the resulting record.
If a spread expression results in a non-record type (e.g., errors), then that
part of the record is simply elided.

The fields of a record expression are evaluated left to right and when
field names collide the rightmost instance of the name determines that
field's value.

For example,
```mdtest-spq
# spq
yield {a:0},{x}, {...r}, {a:0,...r,b:3}
# input
{x:1,y:2,r:{a:1,b:2}}
# expected output
{a:0}
{x:1}
{a:1,b:2}
{a:1,b:3}
```

### Array Expressions

Array literals have the form
```
[ <spec>, <spec>, ... ]
```
where a `<spec>` has one of two forms:
```
<expr>
...<expr>
```

The first form is simply an element in the array, the result of `<expr>`.  The
second form is the `...` spread operator which expects an array or set value as
the result of `<expr>` and inserts all of the values from the result.  If a spread
expression results in neither an array nor set, then the value is elided.

When the expressions result in values of non-uniform type, then the implied
type of the array is an array of type `union` of the types that appear.

For example,
```mdtest-spq
# spq
yield [1,2,3],["hello","world"]
# input
null
# expected output
[1,2,3]
["hello","world"]
```

Arrays can be concatenated using the spread operator,
```mdtest-spq
# spq
yield [...a,...b,5]
# input
{a:[1,2],b:[3,4]}
# expected output
[1,2,3,4,5]
```

### Set Expressions

Set literals have the form
```
|[ <spec>, <spec>, ... ]|
```
where a `<spec>` has one of two forms:
```
<expr>
...<expr>
```

The first form is simply an element in the set, the result of `<expr>`.  The
second form is the `...` spread operator which expects an array or set value as
the result of `<expr>` and inserts all of the values from the result.  If a spread
expression results in neither an array nor set, then the value is elided.

When the expressions result in values of non-uniform type, then the implied
type of the set is a set of type `union` of the types that appear.

Set values are always organized in their "natural order" independent of the order
they appear in the set literal.

For example,
```mdtest-spq
# spq
yield |[3,1,2]|,|["hello","world","hello"]|
# input
null
# expected output
|[1,2,3]|
|["hello","world"]|
```

Arrays and sets can be concatenated using the spread operator,
```mdtest-spq
# spq
yield |[...a,...b,4]|
# input
{a:[1,2],b:|[2,3]|}
# expected output
|[1,2,3,4]|
```

### Map Expressions

Map literals have the form
```
|{ <expr>:<expr>, <expr>:<expr>, ... }|
```
where the first expression of each colon-separated entry is the key value
and the second expression is the value.
When the key and/or value expressions result in values of non-uniform type,
then the implied type of the map has a key type and/or value type that is
a union of the types that appear in each respective category.

For example,
```mdtest-spq
# spq
yield |{"foo":1,"bar"+"baz":2+3}|
# input
null
# expected output
|{"foo":1,"barbaz":5}|
```

### Union Values

A union value can be created with a [cast](expressions.md#casts).  For example, a union of types `int64`
and `string` is expressed as `(int64,string)` and any value that has a type
that appears in the union type may be cast to that union type.
Since 1 is an `int64` and "foo" is a `string`, they both can be
values of type `(int64,string)`, e.g.,
```mdtest-spq
# spq
yield cast(this,<(int64,string)>)
# input
1
"foo"
# expected output
1((int64,string))
"foo"((int64,string))
```

The value underlying a union-tagged value is accessed with the
[`under` function](functions/under.md):
```mdtest-spq
# spq
yield under(this)
# input
1((int64,string))
# expected output
1
```

Union values are powerful because they provide a mechanism to precisely
describe the type of any nested, semi-structured value composed of elements
of different types.  For example, the type of the value `[1,"foo"]` in JavaScript
is simply a generic JavaScript "object".  But in SuperSQL, the type of this
value is an array of union of string and integer, e.g.,
```mdtest-spq
# spq
typeof(this)
# input
[1,"foo"]
# expected output
<[(int64,string)]>
```

## Casts

Type conversion is performed with casts and the built-in [`cast` function](functions/cast.md).

Casts for primitive types have a function-style syntax of the form
```
<type> ( <expr> )
```
where `<type>` is a [type](data-types.md#first-class-types) and `<expr>` is any expression.
In the case of primitive types, the type-value angle brackets
may be omitted, e.g., `<string>(1)` is equivalent to `string(1)`.
If the result of `<expr>` cannot be converted
to the indicated type, then the cast's result is an error value.

For example,
```mdtest-spq {data-layout="stacked"}
# spq
yield int8(this)
# input
1
200
"123"
"200"
# expected output
1(int8)
error({message:"cannot cast to int8",on:200})
123(int8)
error({message:"cannot cast to int8",on:"200"})
```

Casting attempts to be fairly liberal in conversions.  For example, values
of type `time` can be created from a diverse set of date/time input strings
based on the [Go Date Parser library](https://github.com/araddon/dateparse).

```mdtest-spq
# spq
yield time(this)
# input
"May 8, 2009 5:57:51 PM"
"oct 7, 1970"
# expected output
2009-05-08T17:57:51Z
1970-10-07T00:00:00Z
```

Casts of complex or [named types](data-types.md#named-types) may be performed using type values
either in functional form or with `cast`:
```
<type-value> ( <expr> )
cast(<expr>, <type-value>)
```
For example
```mdtest-spq
# spq
type port = uint16

yield <port>(this)
# input
80
8080
# expected output
80(port=uint16)
8080(port=uint16)
```

Casts may be used with complex types as well.  As long as the target type can
accommodate the value, the cast will be recursively applied to the components
of a nested value.  For example,
```mdtest-spq
# spq
cast(this,<[ip]>)
# input
["10.0.0.1","10.0.0.2"]
# expected output
[10.0.0.1,10.0.0.2]
```

and
```mdtest-spq {data-layout="stacked"}
# spq
cast(this,<{ts:time,r:{x:float64,y:float64}}>)
# input
{ts:"1/1/2022",r:{x:"1",y:"2"}}
{ts:"1/2/2022",r:{x:3,y:4}}
# expected output
{ts:2022-01-01T00:00:00Z,r:{x:1.,y:2.}}
{ts:2022-01-02T00:00:00Z,r:{x:3.,y:4.}}
```
