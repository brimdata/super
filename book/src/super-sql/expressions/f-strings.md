## F-Strings

A formatted string (or f-string) is a string literal prefixed with `f`
that includes replacement expressions delimited by curly braces:
```
f"... { <expr> } ... { <expr> } ..."
```
The text starting with `{` and ending at `}` is substituted
with the result of the expression `<expr>`.  As shown, multiple such
expressions may be embedded in an f-string.  If the expression results
in a value that is not a string, then it is implicitly cast to a string.

F-strings may be nested in that the embedded expressions may contain additional
f-strings as in
```
f"an example {upper(f"{foo + bar}")} of nested f-strings"
```
If any expression results in an error, then the value of the f-string is the
first error encountered in left-to-right order.

To represent a literal `{` character inside an f-string, it must be escaped
with a backslash as `\{`.  This escape sequence is valid only in f-strings.

### Examples
---
_Some simple arithmetic_

```mdtest-spq {data-layout="stacked"}
# spq
values f"pi is approximately {numerator / denominator}"
# input
{numerator:22.0, denominator:7.0}
# expected output
"pi is approximately 3.142857142857143"
```
---
_A complex expression with nested f-strings_

```mdtest-spq {data-layout="stacked"}
# spq
values f"oh {this[upper(f"{foo || bar}")]}"
# input
{foo:"hello", bar:"world", HELLOWORLD:"hi!"}
# expected output
"oh hi!"
```
---
_Curly braces can be escaped_

```mdtest-spq
# spq
values f"{this} look like: \{}"
# input
"curly braces"
# expected output
"curly braces look like: {}"
```
