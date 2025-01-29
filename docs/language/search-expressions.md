---
weight: 6
title: Search Expressions
---

Search expressions provide a hybrid syntax between keyword search
and boolean expressions.  In this way, a search is a shorthand for
a "lean forward" style activity where one is interactively exploring
data with ad hoc searches.  All shorthand searches have a corresponding
long form built from the [expression syntax](expressions.md) in combination with the
[search term syntax](search-expressions.md#search-terms) described below.

## Search Patterns

Several styles of string search can be performed with a search expression
(as well as the [`grep` function](functions/grep.md)) using "patterns",
where a pattern is a regular expression, glob, or simple string.

### Regular Expressions

A regular expression is specified in the familiar slash syntax where the
expression begins with a `/` character and ends with a terminating `/` character.
The string between the slashes (exclusive of those characters) is the
regular expression. The format follows the syntax of the
[RE2 regular expression library](https://github.com/google/re2)
and is documented in the
[RE2 Wiki](https://github.com/google/re2/wiki/Syntax).

Regular expressions may be used freely in search expressions, e.g.,
```mdtest-spq
# spq
? /(foo|bar)/
# input
"foo"
{s:"bar"}
{s:"baz"}
{foo:1}
# expected output
"foo"
{s:"bar"}
{foo:1}
```
Regular expressions may also appear in the [`grep`](functions/grep.md),
[`regexp`](functions/regexp.md), and [`regexp_replace`](functions/regexp_replace.md) functions:
```mdtest-spq {data-layout="stacked"}
# spq
yield {ba_start:grep(/^ba.*/, s),last_s_char:regexp(/(.)$/,s)[1]}
# input
"foo"
{s:"bar"}
{s:"baz"}
{foo:1}
# expected output
{ba_start:false,last_s_char:error("missing")}
{ba_start:true,last_s_char:"r"}
{ba_start:true,last_s_char:"z"}
{ba_start:false,last_s_char:error("missing")}
```

### Globs

Globs provide a convenient short-hand for regular expressions and follow
the familiar pattern of "file globbing" supported by Unix shells.
Globs are a simple, special case that utilize only the `*` wildcard.

Valid glob characters include `a` through `z`, `A` through `Z`,
any valid string escape sequence
(along with escapes for `*`, `=`, `+`, `-`), and the unescaped characters:
```
_ . : / % # @ ~
```
A glob must begin with one of these characters or `*` then may be
followed by any of these characters, `*`, or digits `0` through `9`.

{{% tip "Note" %}}

These rules do not allow for a leading digit.

{{% /tip %}}

For example, a prefix match is easily accomplished via `prefix*`, e.g.,
```mdtest-spq
# spq
? b*
# input
"foo"
{s:"bar"}
{s:"baz"}
{foo:1}
# expected output
{s:"bar"}
{s:"baz"}
```

Likewise, a suffix match may be performed as follows:
```mdtest-spq
# spq
? *z
# input
"foo"
{s:"bar"}
{s:"baz"}
{foo:1}
# expected output
{s:"baz"}
```
and
```mdtest-spq
# spq
? *a*
# input
"foo"
{s:"bar"}
{s:"baz"}
{a:1}
# expected output
{s:"bar"}
{s:"baz"}
{a:1}
```

Globs may also appear in the [`grep` function](functions/grep.md):
```mdtest-spq
# spq
yield grep(ba*, s)
# input
"foo" {s:"bar"} {s:"baz"} {foo:1}
# expected output
false
true
true
false
```

Note that a glob may look like multiplication but context disambiguates
these conditions, e.g.,
```
a*b
```
is a glob match for any matching string value in the input, but
```
a*b==c
```
is a Boolean comparison between the product `a*b` and `c`.

## Search Logic

The search patterns described above can be combined with other "search terms"
using Boolean logic to form search expressions.

{{% tip "Note" %}}

When processing [Super Binary](../formats/bsup.md) data, the SuperDB runtime performs a multi-threaded
Boyer-Moore scan over decompressed data buffers before parsing any data.
This allows large buffers of data to be efficiently discarded and skipped when
searching for rarely occurring values.  For a [SuperDB data lake](../lake/format.md),
a planned feature will use [Super Columnar](../formats/csup.md) files to further accelerate searches.

{{% /tip %}}

### Search Terms

A "search term" is one of the following;
* a regular expression as described above,
* a glob as described above,
* a keyword,
* any literal of a primitive type, or
* expression predicates.

#### Regular Expression Search Term

A regular expression `/re/` is equivalent to
```
grep(/re/, this)
```
but shorter and easier to type in a search expression.

For example,
```
? /(foo|bar.*baz.*\.com)/
```
Searches for any string that begins with `foo` or `bar` has the string
`baz` in it and ends with `.com`.

#### Glob Search Term

A glob search term `<glob>` is equivalent to
```
grep(<glob>, this)
```
but shorter and easier to type in a search expression.

For example,
```
? foo*baz*.com
```
Searches for any string that begins with `foo` has the string
`baz` in it and ends with `.com`.

#### Keyword Search Term

Keywords and string literals are equivalent search terms so it is often
easier to quote a string search term instead of using escapes in a keyword.
Keywords are useful in interactive workflows where searches can be issued
and modified quickly without having to type matching quotes.

Keyword search has the look and feel of Web search or email search.

Valid keyword characters include `a` through `z`, `A` through `Z`,
any valid string escape sequence
(along with escapes for `*`, `=`, `+`, `-`), and the unescaped characters:
```
_ . : / % # @ ~
```
A keyword must begin with one of these characters then may be
followed by any of these characters or digits `0` through `9`.

A keyword search is equivalent to
```
grep(<keyword>, this)
```
where `<keyword>` is the quoted string-literal of the unquoted string.
For example,
```
search foo
```
is equivalent to
```
where grep("foo", this)
```

Note that the "search" keyword may be omitted.
For example, the simplest SuperPipe query is perhaps a single keyword search, e.g.,
```
? foo
```
As above, this query searches the implied input for values that
contain the string "foo".

#### String Literal Search Term

A string literal as a search term is simply a search for that string and is
equivalent to
```
grep(<string>, this)
```
For example,
```
search "foo"
```
is equivalent to
```
where grep("foo", this)
```

{{% tip "Note" %}}

This equivalency between keyword search terms and grep semantics
will change in the near future when we add support for full-text search.
In this case, grep will still support substring match but keyword search
will match segmented words from string fields.

{{% /tip %}}

#### Non-String Literal Search Term

Search terms representing non-string values search for both an exact
match for the given value as well as a string search for the term exactly
as it appears as typed.  Such values include:
* integers,
* floating point numbers,
* time values,
* durations,
* IP addresses,
* networks,
* bytes values, and
* type values.

A search for a value `<value>` represented as the string `<string>` is
equivalent to
```
<value> in this or grep(<string>, this)
```
For example,
```
search 123 and 10.0.0.1
```
which can be abbreviated
```
? 123 10.0.0.1
```
is equivalent to
```
where (123 in this or grep("123", this))
  and (10.0.0.1 in this or grep("10.0.0.1", this))
```

Complex values are not supported as search terms but may be queried with
the "in" operator, e.g.,
```
{s:"foo"} in this
```

#### Predicate Search Term

Any Boolean-valued [function](functions/_index.md) like `is`, `has`,
`grep`, etc. and any [comparison expression](expressions.md#comparisons)
may be used as a search term and mixed into a search expression.

For example,
```
? is(<foo>) has(bar) baz x==y+z timestamp > 2018-03-24T17:17:55Z
```
is a valid search expression but
```
? /foo.*/ x+1
```
is not.

### Boolean Logic

Search terms may be combined into boolean expressions using logical operators
`and`, `or`, `not`, and `!`.  `and` may be elided; i.e., concatenation of
search terms is a logical `and`.  `not` (and its equivalent `!`) has highest
precedence and `and` has precedence over `or`.  Parentheses may be used to
override natural precedence.

Note that the concatenation form of `and` is not valid in standard expressions and
is available only in search expressions.
Concatenation is convenient in interactive sessions but it is best practice to
explicitly include the `and` operator when composing saved queries planned for
re-use and sharing.

For example,
```
? not foo bar or baz
```
means
```
((not grep("foo")) and grep("bar)) or grep("baz")
```
while
```
? foo (bar or baz)
```
means
```
grep("foo") and (grep("bar") or grep("baz"))
```
