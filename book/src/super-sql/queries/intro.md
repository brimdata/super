## Queries

The syntactical structure of a pipe query consists of
* an optional concatentation of [declarations](declarations/intro.md), 
  followed by
* a sequence of [pipe operators](../operators/intro.md)
  separated by the pipe character (`|`).

Any valid [SQL query](../sql/intro.md) may appear as a pipe operator and thus
be embedded in a pipe query.

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
and digits `[0-9]`, but may not start with a digit.  An identifier cannot be
`true`, `false`, or `null` or another SuperSQL keyword.

To express an identifier that does not meet the requiremented of an
unquoted identifier, arbitraray text may be quoted inside of backticks.

XXX explain escapes (point to strings)

> _can put backtick in inside backtick string with Unicode escape `\u0060`._

XXX list the keywords somewhere

an indentifier may be
 enclosed in backticks (`` ` ``).

XXX explain

backtick identifiers

XXX somewhere explain string quotes, backtick identifiers, and
double-quote identifiers in SQL context all in the same place (here)
and point to the respective definitions of these things (like types/string)
and (expressions/intro#id)
