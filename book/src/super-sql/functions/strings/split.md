### Function

&emsp; **split** &mdash; slice a string into an array of strings

### Synopsis

```
split(s: string, sep: string) -> [string]
```

### Description

The _split_ function slices string `s` into all substrings separated by the
string `sep` appearing in `s` and returns an array of the substrings
spanning those separators.

### Examples

Split a semi-colon delimited list of fruits:
```mdtest-spq
# spq
values split(this,";")
# input
"apple;banana;pear;peach"
# expected output
["apple","banana","pear","peach"]
```

Split a comma-separated list of IPs and cast the array of strings to an
array of IPs:
```mdtest-spq
# spq
values cast(split(this,","),<[ip]>)
# input
"10.0.0.1,10.0.0.2,10.0.0.3"
# expected output
[10.0.0.1,10.0.0.2,10.0.0.3]
```
