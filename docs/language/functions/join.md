### Function

&emsp; **join** &mdash; concatenate array of strings with a separator

### Synopsis

```
join(val: [string], sep: string) -> string
```

### Description

The _join_ function concatenates the elements of string array `val` to create a single
string. The string `sep` is placed between each value in the resulting string.

#### Example:

Join a symbol array of strings:
```mdtest-spq
# spq
values join(this, ",")
# input
["a","b","c"]
# expected output
"a,b,c"
```

Join non-string arrays by first casting:
```mdtest-spq
# spq
values join(cast(this, <[string]>), "...")
# input
[1,2,3]
[10.0.0.1,10.0.0.2]
# expected output
"1...2...3"
"10.0.0.1...10.0.0.2"
```
