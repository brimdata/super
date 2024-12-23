### Aggregate Function

&emsp; **and** &mdash; logical AND of input values

### Synopsis
```
and(bool) -> bool
```

### Description

The _and_ aggregate function computes the logical AND over all of its input.

### Examples

Anded value of simple sequence:
```mdtest-command
echo 'true false true' | super -z -c 'and(this)' -
```

```mdtest-output
false
```

Continuous AND of simple sequence:
```mdtest-command
echo 'true false true' | super -z -c 'yield and(this)' -
```

```mdtest-output
true
false
false
```

Unrecognized types are ignored and not coerced for truthiness:
```mdtest-command
echo 'true "foo" 0 false true' | super -z -c 'yield and(this)' -
```

```mdtest-output
true
true
true
false
false
```

AND of values grouped by key:
```mdtest-command
echo '{a:true,k:1} {a:true,k:1} {a:true,k:2} {a:false,k:2}' |
  super -z -c 'and(a) by k |> sort' -
```

```mdtest-output
{k:1,and:true}
{k:2,and:false}
```
