### Function

&emsp; **regexp** &mdash; perform a regular expression search on a string

### Synopsis

```
regexp(re: string|regexp, s: string) -> any
```

### Description
The _regexp_ function returns an array of strings holding the text
of the left most match of the regular expression `re`, which can be either
a string value or a [regular expression](../search-expressions.md#regular-expressions),
and the matches of each parenthesized subexpression (also known as capturing
groups) if there are any. A null value indicates no match.

### Examples

Regexp returns an array of the match and its subexpressions:
```mdtest-spq
# spq
values regexp(/foo(.?) (\w+) fr.*/, this)
# input
"seafood fool friend"
# expected output
["food fool friend","d","fool"]
```

A null is returned if there is no match:
```mdtest-spq
# spq
values regexp("bar", this)
# input
"foo"
# expected output
null::[string]
```
