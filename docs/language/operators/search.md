### Operator

&emsp; **search** &mdash; select values based on a search expression

### Synopsis
```
[search] <sexpr>
```
### Description

The `search` operator filters its input by applying a [search expression](../search-expressions.md) `<sexpr>`
to each input value and dropping each value for which the expression evaluates
to `false` or to an error.

The `search` keyword is optional since it is an
[implied operator](../pipeline-model.md#implied-operators).

When Zed queries are run interactively, it is convenient to be able to omit
the "search" keyword, but when search filters appear in Zed source files,
it is good practice to include the optional keyword.

### Examples

_A simple keyword search for "world"_
```mdtest-command
echo '"hello, world" "say hello" "goodbye, world"' | super query -z -c 'search world' -
```
=>
```mdtest-output
"hello, world"
"goodbye, world"
```
Search can utilize _arithmetic comparisons_
```mdtest-command
echo '1 2 3' | super query -z -c 'search this >= 2' -
```
=>
```mdtest-output
2
3
```
_The "search" keyword may be dropped_
```mdtest-command
echo '1 2 3' | super query -z -c '2 or 3' -
```
=>
```mdtest-output
2
3
```
_A search with [Boolean logic](../search-expressions.md#boolean-logic)_
```mdtest-command
echo '1 2 3' | super query -z -c 'search this >= 2 AND this <= 2' -
```
=>
```mdtest-output
2
```
_The AND operator may be omitted through predicate concatenation_
```mdtest-command
echo '1 2 3' | super query -z -c 'search this >= 2 this <= 2' -
```
=>
```mdtest-output
2
```
_Concatenation for keyword search_
```mdtest-command
echo '"foo" "foo bar" "foo bar baz" "baz"' | super query -z -c 'foo bar' -
```
=>
```mdtest-output
"foo bar"
"foo bar baz"
```
_Search expressions match fields names too_
```mdtest-command
echo '{foo:1} {bar:2} {foo:3}' | super query -z -c foo -
```
=>
```mdtest-output
{foo:1}
{foo:3}
```
_Boolean functions may be called_
```mdtest-command
echo '1 "foo" 10.0.0.1' | super query -z -c 'search is(<int64>)' -
```
=>
```mdtest-output
1
```
_Boolean functions with Boolean logic_
```mdtest-command
echo '1 "foo" 10.0.0.1' | super query -z -c 'search is(<int64>) or is(<ip>)' -
```
=>
```mdtest-output
1
10.0.0.1
```
