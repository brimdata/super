### Function

&emsp; **rune_len** &mdash; length of a string in Unicode code points

### Synopsis

```
rune_len(s: string) -> int64
```

### Description

The `rune_len` function returns the number of Unicode code points in
the argument string `s`.  Since strings are always encoded as UTF-8,
this length is the same as the number of UTF-8 characters.

### Examples

---

_The length in UTF-8 characters of a smiley is 1_

```mdtest-spq
# spq
values rune_len(this)
# input
"hello"
"😎"
# expected output
5
1
```

---

_The length in bytes of a smiley is 4_

```mdtest-spq
# spq
values len(this::bytes)
# input
"hello"
"😎"
# expected output
5
4
```
