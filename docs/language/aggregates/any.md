### Aggregate Function

&emsp; **any** &mdash; select an arbitrary input value

### Synopsis
```
any(any) -> any
```

### Description

The _any_ aggregate function returns an arbitrary element from its input.
The semantics of how the item is selected is not defined.

### Examples

Any picks the first one in this scenario but this behavior is undefined:
```mdtest-spq
# spq
any(this)
# input
1
2
3
4
# expected output
1
```

Continuous any over a simple sequence:
```mdtest-spq
# spq
yield any(this)
# input
1 2 3 4
# expected output
1
1
1
1
```

Any is not sensitive to mixed types as it just picks one:
```mdtest-spq
# spq
any(this)
# input
"foo"
1
2
3
# expected output
"foo"
```

Pick from groups bucketed by key:
```mdtest-spq
# spq
any(a) by k | sort
# input
{a:1,k:1}
{a:2,k:1}
{a:3,k:2}
{a:4,k:2}
# expected output
{k:1,any:1}
{k:2,any:3}
```
