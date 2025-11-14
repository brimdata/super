### Function

&emsp; **regexp** &mdash; perform a regular expression search on a string

### Synopsis

```
regexp(re: string, s: string) -> any
```

### Description

The `regexp` function returns an array of strings holding the text
of the left most match of the regular expression `re`, which is
a [regular expression](../../queries.md#regular-expression),
and the matches of each parenthesized subexpression (also known as capturing
groups) if there are any. A null value indicates no match.

### Examples

---

_Regexp returns an array of the match and its subexpressions_

```mdtest-spq
# spq
values regexp(r'foo(.?) (\w+) fr.*', this)
# input
"seafood fool friend"
# expected output
["food fool friend","d","fool"]
```

---

_A null is returned if there is no match_

```mdtest-spq
# spq
values regexp("bar", this)
# input
"foo"
# expected output
null::[string]
```
