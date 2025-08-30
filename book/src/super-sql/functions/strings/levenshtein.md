### Function

&emsp; **levenshtein** &mdash; Levenshtein distance

### Synopsis

```
levenshtein(a: string, b: string) -> int64
```

### Description

The `levenshtein` function computes the
[Levenshtein distance](https://en.wikipedia.org/wiki/Levenshtein_distance)
between strings `a` and `b`.

### Examples

---

```mdtest-spq
# spq
values levenshtein(a, b)
# input
{a:"kitten",b:"sitting"}
# expected output
3
```
