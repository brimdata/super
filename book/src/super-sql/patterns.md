## Patterns

For ease of use, several operators utilize a syntax for string entities
outside of expression syntax where quotation marks for such entities
may be conveniently elided.

For example, when sourcing data from a file on the file system, the file
path can be expressed as a [text entity](#text-entity) and need not be quoted:
```
from file.json | ...
```

Likewise, in the search operator, the syntax for a [regular expression](#regular-expression)
search can be specified as
```
search /\w+(foo|bar)/
```
instead of the `regexp` function call required in expression context
```
where regexp(r'\w+(foo|bar)')
```

### Regular Expression

A regular expression follows the syntax and semantics of the
[RE2 regular expression library](https://github.com/google/re2)
and is documented in the
[RE2 Wiki](https://github.com/google/re2/wiki/Syntax).

When used in an expression, e.g., as a parameter to a function, the
RE2 text is simply passed as a string, e.g.,
```
regexp('foo|bar')
```

But when used outside of expressions where an explicit indication of
a regular expression is required (e.g., in a
[`search`](operators/search.md) or 
[`from`](operators/from.md) operator), the RE2 is instead
prefixed and suffixed with a `/`, e.g.,
```
/foo|bar/
```
matches the string `"foo"` or `"bar"`.

### Glob

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

### Text Entity

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
e.g., containing a `:` or `&`, then it can simply be quoted.

