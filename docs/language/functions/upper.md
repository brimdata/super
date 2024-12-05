### Function

&emsp; **upper** &mdash; convert a string to upper case

### Synopsis

```
upper(s: string) -> string
```

### Description

The _upper_ function converts all lower case Unicode characters in `s`
to upper case and returns the result.

### Examples

```mdtest-command
echo '"Super JSON"' | super -z -c 'yield upper(this)' -
```

```mdtest-output
"SUPER JSON"
```

[Slices](../expressions.md#slices) can be used to uppercase a subset of a string as well.

```mdtest-command
echo '"super JSON"' |
  super -z -c 'func capitalize(str): (
           upper(str[0:1]) + str[1:]
         )
         yield capitalize(this)
  ' -
```

```mdtest-output
"Super JSON"
```
