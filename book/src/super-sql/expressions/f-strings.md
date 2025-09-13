## F-Strings

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
values f"pi is approximately {numerator / denominator}"
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
values f"oh {this[upper(f"{foo + bar}")]}"
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
values f"{this} look like: \{ }"
# input
"brackets"
# expected output
"brackets look like: { }"
```