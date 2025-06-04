## Function Calls

Functions perform stateless transformations of their input value to their return
value and utilize call-by value semantics with positional and unnamed arguments.

For example,
```mdtest-spq
# spq
values pow(2,3), lower("ABC")+upper("def"), typeof(1)
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


#### Function Reference

XXX todo `&foo` convention

#### Lambda Expression

