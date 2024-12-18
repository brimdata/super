### Function

&emsp; **levenshtein** &mdash; Levenshtein distance

### Synopsis

```
levenshtein(a: string, b: string) -> int64
```

### Description

The _levenshtein_ function computes the [Levenshtein
distance](https://en.wikipedia.org/wiki/Levenshtein_distance) between strings
`a` and `b`.

### Examples

```mdtest-command
echo '{a:"kitten",b:"sitting"}' | super -z -c 'yield levenshtein(a, b)' -
```

```mdtest-output
3
```
