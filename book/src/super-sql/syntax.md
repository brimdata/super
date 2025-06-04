## Syntax

The syntactical structure of a pipe query consists of
* an optional concatentation of [declarations](declarations/intro.md), 
  followed by
* a sequence of [pipe operators](operators/intro.md)
  separated by a pipe symbol (`|` or `|>`).

Any valid [SQL query](../sql/intro.md) may appear as a pipe operator and thus
be embedded in a pipe query.

Operator sequences may be parenthesized and nested to form [scopes](#scope).

Operators utilize expressions in a myriad of composable variations to
perform their computations and all expressions
share a common [expression syntax](expressions/intro.md).
While operators consume a sequence of values, the expressions embedded
within an operator are typically evaluated once for each value processed
by the operator.

### Scope

A scope is formed by enclosing a set of declarations along with an operator
sequence in the parentheses having the structure:
```
(
    <declarations>
    <operators>
)
```
Scope blocks may appear anywhere a [pipe operator](operators/intro.md) may appear,
as a [subquery](expressions/subqueries.md) in an expression,
as a [lateral subquery](expressions/subqueries.md#lateral-subquery), or
as the body of [declared operator](declarations/operators.md).

Any query can be enclosed in parentheses and additional declarations
may appear at the beginning of the parenthesized query.
The parenthesized entity forms a
[lexical scope](https://en.wikipedia.org/wiki/Scope_(computer_science)#Lexical_scope)
and the bindings created by declarations
within the scope are reachable only within that scope inclusive
of other scopes defined within the scope.

The topmost scope is the global scope where all declared identifiers
are available everywhere and does not include parentheses.

Note that this lexical scope refers only to the declared identifiers.  Scoping
of references to data input is defined by
[dataflow scoping](intro.md#dataflow-scoping) and
[relational scoping](intro.md#relational-scoping).

For example,
```
const pi=3.14
values pi
```
emits the value of the constant `pi`, but
```
( 
  const pi=3.14
  values pi
)
| values this+pi
```
emits `error("missing")` because the second reference to `pi` does not
the declared constant as it's in the outer scope,
and thus it is bound `this.pi` via dataflow scoping,
which does not exist at entry to the second [`values`](operators/values.md) operator.

### Identifiers

Identifiers are names that arise in many syntactical structures and
may be any sequence of UTF-8 characters.  When not quoted,
an identifier may be comprised of Unicode letters, `$`, `_`,
and digits `[0-9]`, but may not start with a digit.

To express an identifier that does not meet the requiremented of an
unquoted identifier, arbitraray text may be quoted inside of backtick (`` ` ``)
quotes.
Escape sequences in backtick-quoted identifiers are interpreted as in
[string literals](../types/string.md).  In particular, a backtick (`` ` ``)
character may be included in a backtick string with Unicode escape `\u0060`.

In SQL expressions, identifiers may also be enclosed in double-quoted strings.

An unquoted identifier cannot be `true`, `false`, `null`, or a SQL keyword.

> TODO: check this and list the identifiers. SQLGuard?

### Patterns

For ease of use, several operators utilize a syntax for string entities
outside of expression syntax where quotation marks for such entities
may be conveniently elided.

For example, when sourcing data from a file on the file system, the file
path can be expressed as a [text entity](#text-entity) and need not be quoted:
```
from file.json | ...
```

Likewise, in the [`search`](operators/search.md) operator, the syntax for a
[regular expression](#regular-expression) search can be specified as
```
search /\w+(foo|bar)/
```
whereas an explicit function call like `regexp` must be invokced to utilize
regular expressions in expressions as in
```
where len(regexp(r'\w+(foo|bar)', this)) > 0
```

#### Regular Expression

Regular expressions follow the syntax and semantics of the
[RE2 regular expression library](https://github.com/google/re2),
which is documented in the
[RE2 Wiki](https://github.com/google/re2/wiki/Syntax).

When used in an expression, e.g., as a parameter to a function, the
RE2 text is simply passed as a string, e.g.,
```
regexp('foo|bar', this)
```

To avoid having to add escaping that would otherwise be necessary to
represent a regular expression as a string, prefix with `r`, e.g.,
```
regexp(r'\w+(foo|bar)', this)
```

But when used outside of expressions where an explicit indication of
a regular expression is required (e.g., in a
[`search`](operators/search.md) or
[`from`](operators/from.md#database-operation) operator), the RE2 is instead
prefixed and suffixed with a `/`, e.g.,
```
/foo|bar/
```
matches the string `"foo"` or `"bar"`.

#### Glob

Globs provide a convenient short-hand for regular expressions and follow
the familiar pattern of "file globbing" supported by Unix shells.
Globs are a simple, special case that utilize only the `*` wildcard.

Like regular expressions, globs may be used in
a [`search`](operators/search.md) operator or a
[`from`](operators/from.md) operator.

Valid glob characters include letters, digits (excepting the leading character),
any valid string escape sequence
(along with escapes for `*`, `=`, `+`, `-`), and the unescaped characters:
```
_ . : / % # @ ~ *
```
A glob cannot begin with a digit.

#### Text Entity

A text entity represents a string where quotes can be omitted for
certain common use cases regarding URLs and file paths.

Text entities are syntactically valid as targets of a
[`from`](operators/from.md) operator and as named arguments
to `from` and the
[`load`](operators/load.md) operator.

Specifically, a text entity is one of:
* a string literal (double quoted, single quoted, or raw string),
* a path consisting of a sequence of characters consisting of letters, digits, `_`,  `$`,  `.`, and `/`, or
* a simple URL consisting of a sequence of characters beginning with `http://` or `https://`,  followed by dotted strings of letters, digits, `-`, and `_`, and in turn optionally followed by `/` and a sequence of characters consisting of letters, digits, `_`, `$`, `.`, and `/`.

If a URL does not meet the constraints of the simple URL rule,
e.g., containing a `:` or `&`, then it must be quoted.

### Comments

Single-line comments are SQL style begin with two dashes `--` and end at the
subsequent newline.

Multi-line comments are C style and begin with `/*` and end with `*/`.

```mdtest-spq
# spq
values 1, 2 -- , 3
/*
| aggregate sum(this)
*/
| aggregate sum(this / 2.0)
# input
null
# expected output
1.5
```
