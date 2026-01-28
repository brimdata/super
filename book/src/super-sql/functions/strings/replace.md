# replace

replace one string for another

## Synopsis

```
replace(s: string, old: string, new: string) -> string
```

## Description

The `replace` function substitutes all instances of the string `old`
that occur in string `s` with the string `new`.

## Example

---

```mdtest-spq
# spq
values replace(this, "oink", "moo")
# input
"oink oink oink"
# expected output
"moo moo moo"
```
