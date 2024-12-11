### Function

&emsp; **regexp_replace** &mdash; replace regular expression matches in a string

### Synopsis

```
regexp_replace(s: string, re: string|regexp, new: string) -> string
```

### Description

The _regexp_replace_ function substitutes all characters matching the
[regular expression](../search-expressions.md#regular-expressions) `re` in string `s` with
the string `new`.

Variables in `new` are replaced with corresponding matches drawn from `s`.
A variable is a substring of the form `$name` or `${name}`, where `name` is a non-empty
sequence of letters, digits, and underscores. A purely numeric name like `$1` refers
to the submatch with the corresponding index; other names refer to capturing
parentheses named with the `(?P<name>...)` syntax. A reference to an out of range or
unmatched index or a name that is not present in the regular expression is replaced
with an empty string.

In the `$name` form, `name` is taken to be as long as possible: `$1x` is equivalent to
`${1x}`, not `${1}x`, and, `$10` is equivalent to `${10}`, not `${1}0`.

To insert a literal `$` in the output, use `$$` in the template.

#### Examples:

Replace regular expression matches with a letter:

```mdtest-command
echo '"-ab-axxb-"' | super -z -c 'yield regexp_replace(this, /ax*b/, "T")' -
```

```mdtest-output
"-T-T-"
```

Replace regular expression matches using numeric references to submatches:

```mdtest-command
echo '"option: value"' |
  super -z -c 'yield regexp_replace(this,/(\w+):\s+(\w+)$/,"$1=$2")' -
```

```mdtest-output
"option=value"
```

Replace regular expression matches using named references:

```mdtest-command
echo '"option: value"' |
  super -z -c 'yield regexp_replace(
                 this,
                 /(?P<key>\w+):\s+(?P<value>\w+)$/,
                 "$key=$value")
  ' -
```

```mdtest-output
"option=value"
```

Wrap a named reference in curly braces to avoid ambiguity:

```mdtest-command
echo '"option: value"' |
  super -z -c 'yield regexp_replace(
                 this,
                /(?P<key>\w+):\s+(?P<value>\w+)$/,
                "$key=${value}AppendedText")
  ' -
```

```mdtest-output
"option=valueAppendedText"
```
