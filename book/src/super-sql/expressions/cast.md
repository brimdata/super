## Cast

Cast expressions
(just explain syntax to get to cast function)


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
values this::int8
# input
1
200
"123"
"200"
# expected output
1::int8
error({message:"cannot cast to int8",on:200})
123::int8
error({message:"cannot cast to int8",on:"200"})
```

Casting attempts to be fairly liberal in conversions.  For example, values
of type `time` can be created from a diverse set of date/time input strings
based on the [Go Date Parser library](https://github.com/araddon/dateparse).

```mdtest-spq
# spq
values this::time
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

values this::<port>
# input
80
8080
# expected output
80::(port=uint16)
8080::(port=uint16)
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
