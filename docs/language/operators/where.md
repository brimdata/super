### Operator

&emsp; **where** &mdash; select values based on a Boolean expression

### Synopsis
```
[where] <expr>
```
### Description

The `where` operator filters its input by applying a Boolean expression `<expr>`
to each input value and dropping each value for which the expression evaluates
to `false` or to an error.

The `where` keyword is optional since it is an
[implied operator](../pipeline-model.md#implied-operators).

The "where" keyword requires a boolean-valued expression and does not support
[search expressions](../search-expressions.md).  Use the
[search operator](search.md) if you want search syntax.

When SuperPipe queries are run interactively, it is highly convenient to be able to omit
the "where" keyword, but when where filters appear in query source files,
it is good practice to include the optional keyword.

### Examples

_An arithmetic comparison_
```mdtest-spq
# spq
where this >= 2
# input
1
2
3
# expected output
2
3
```

_The "where" keyword may be dropped_
```mdtest-spq
# spq
this >= 2
# input
1
2
3
# expected output
2
3
```

_A filter with Boolean logic_
```mdtest-spq
# spq
where this >= 2 AND this <= 2
# input
1
2
3
# expected output
2
```

_A filter with array containment logic_
```mdtest-spq
# spq
where this in [1,4]
# input
1
2
3
4
# expected output
1
4
```

_A filter with inverse containment logic_
```mdtest-spq
# spq
where ! (this in [1,4])
# input
1
2
3
4
# expected output
2
3
```

_Boolean functions may be called_
```mdtest-spq
# spq
where is(<int64>)
# input
1
"foo"
10.0.0.1
# expected output
1
```

_Boolean functions with Boolean logic_
```mdtest-spq
# spq
where is(<int64>) or is(<ip>)
# input
1
"foo"
10.0.0.1
# expected output
1
10.0.0.1
```
