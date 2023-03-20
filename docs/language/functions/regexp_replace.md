### Function

&emsp; **regexp_replace** &mdash; replace regular expression matches in a string

### Synopsis

```
regexp_replace(s: string, re: string|regexp, new: string) -> string
```
### Description

The _regexp_replace_ function substitutes all characters matching the
[regular expression](../overview.md#regular-expressions) `re` in string `s` with
the string `new`.

Variables in `new` are replaced with corresponding matches drawn from `s`.
A variable is a substring of the form $name or ${name}, where name is a non-empty
sequence of letters, digits, and underscores. A purely numeric name like $1 refers
to the submatch with the corresponding index; other names refer to capturing
parentheses named with the `(?P<name>...)` syntax. A reference to an out of range or
unmatched index or a name that is not present in the regular expression is replaced
with an empty slice.

In the $name form, name is taken to be as long as possible: $1x is equivalent to
${1x}, not ${1}x, and, $10 is equivalent to ${10}, not ${1}0.

To insert a literal $ in the output, use $$ in the template.

#### Example:

```mdtest-command
echo '"-ab-axxb-"' | zq -z 'yield regexp_replace(this, /a(x*)b/, "T")' -
```
=>
```mdtest-output
"-T-T-"
```
