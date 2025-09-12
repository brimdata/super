## Queries

The syntactical structure of a pipe query consists of
* an optional concatentation of [declarations](declarations/intro.md), 
  followed by
* a sequence of [pipe operators](../operators/intro.md)
  separated by a pipe symbol (`|` or `|>`).

Any valid [SQL query](../sql/intro.md) may appear as a pipe operator and thus
be embedded in a pipe query.

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

### Scoping

A declaration binds a name expressed as an [identifier](#identifiers) to
* a [constant value](declarations/constants.md),
* a [type](declarations/types.md),
* an [operator](declarations/operators.md), or
* a [function](declarations/functios.md).

Any query can be enclosed in parentheses and additional declarations
may appear at the beginning of the parenthesized query.
The parenthesized entity forms a
[lexical scope](https://en.wikipedia.org/wiki/Scope_(computer_science)#Lexical_scope)
and the bindings created by declarations
within the scope are reachable only within that scope inclusive
of other scopes defined within the scope.

The topmost scope is the global scope where all declared identifiers
are available everywhere.

Note that this lexical scope refers only to the declared identifiers.  Scoping
of references to data input is defined by
[dataflow scoping](../intro.md#dataflow-scoping) and
[relational scoping](../intro.md#relational-scoping).

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
which does not exist.

### Identifiers

Identifiers are names that define many entities in a query and
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

An unquoted identifier cannot be `true`, `false`, or `null` or a SQL keyword.

