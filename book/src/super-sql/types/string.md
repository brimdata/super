### Strings

The `string` type represents any valid
[UTF-8 string](https://en.wikipedia.org/wiki/UTF-8).

A string is formed by enclosing the string's unicode characters in
quotation marks whereby the following escape sequences allowed:

| Sequence | Unicode Character      |
|----------|------------------------|
| `\"`     | quotation mark  U+0022 |
| `\\`     | reverse solidus U+005C |
| `\/`     | solidus         U+002F |
| `\b`     | backspace       U+0008 |
| `\f`     | form feed       U+000C |
| `\n`     | line feed       U+000A |
| `\r`     | carriage return U+000D |
| `\t`     | tab             U+0009 |
| `\uXXXX` |                 U+XXXX |

The backslash character (`\`) and the control characters (U+0000 through U+001F)
must be escaped.

In SQL expressions, the quotation mark is a single quote character (`'`) 
and in pipe expressions, the quoatation mark may be either single quote or
double quote (`"`).

In single-quote strings, the single-quote character must 
be escaped and in double-quote strings, the double-quote character must be
escaped.

#### Examples

```mdtest-spq
# spq
values 'hello, world', len('foo'), "SuperDB", "\"quoted\""
# input
null
# expected output
"hello, world"
3
"SuperDB"
"quoted"
```
---

```mdtest-spq
# spq
select 'x' as s, "x" as x from (values (1),(2)) T(x)
# input
null
# expected output
{s:"x",x:1}
{s:"x",x:2}
```
