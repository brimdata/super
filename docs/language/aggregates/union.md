### Aggregate Function

&emsp; **union** &mdash; set union of input values

### Synopsis
```
union(any) -> |[any]|
```

### Description

The _union_ aggregate function computes a set union of its input values.
If the values are of uniform type, then the output is a set of that type.
If the values are of mixed typs, the the output is a set of union of the
types encountered.

### Examples

Create a set of values from a simple sequence:
```mdtest-command
echo '1 2 3 3' | super -z -c 'union(this)' -
```

```mdtest-output
|[1,2,3]|
```

Create sets continuously from values in a simple sequence:
```mdtest-command
echo '1 2 3 3' | super -z -c 'yield union(this)' -
```

```mdtest-output
|[1]|
|[1,2]|
|[1,2,3]|
|[1,2,3]|
```

Mixed types create a union type for the set elements:
```mdtest-command
echo '1 2 3 "foo"' | super -z -c 'set:=union(this) |> yield this,typeof(set)' -
```

```mdtest-output
{set:|[1,2,3,"foo"]|}
<|[(int64,string)]|>
```

Create sets of values bucketed by key:
```mdtest-command
echo '{a:1,k:1} {a:2,k:1} {a:3,k:2} {a:4,k:2}' |
  super -z -c 'union(a) by k |> sort' -
```

```mdtest-output
{k:1,union:|[1,2]|}
{k:2,union:|[3,4]|}
```
