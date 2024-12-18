### Function

&emsp; **replace** &mdash; replace one string for another

### Synopsis

```
replace(s: string, old: string, new: string) -> string
```

### Description

The _replace_ function substitutes all instances of the string `old`
that occur in string `s` with the string `new`.

#### Example:

```mdtest-command
echo '"oink oink oink"' | super -z -c 'yield replace(this, "oink", "moo")' -
```

```mdtest-output
"moo moo moo"
```
