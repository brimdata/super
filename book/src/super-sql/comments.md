## Comments

Single-line comments are SQL style begin with two dashes `--` and end at the
subsequent newline.

Multi-line comments are C style and begin with `/*` and end with `*/`.

```mdtest-spq
# spq
values 1, 2 -- , 3
/*
| aggregate sum(this)
*/
| aggregate sum(this / 2.0)
# input
null
# expected output
1.5
```
