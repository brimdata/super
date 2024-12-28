### Function

&emsp; **unflatten** &mdash; transform an array of key/value records into a
record.

### Synopsis

```
unflatten(val: [{key:string|[string],value:any}]) -> record
```

### Description
The _unflatten_ function converts the key/value records in array `val` into
a single record. _unflatten_ is the inverse of _flatten_, i.e., `unflatten(flatten(r))`
will produce a record identical to `r`.

### Examples
Simple:
```mdtest-command
echo '[{key:"a",value:1},{key:["b"],value:2}]' |
  super -z -c 'yield unflatten(this)' -
```

```mdtest-output
{a:1,b:2}
```

Flatten to unflatten:
```mdtest-command
echo '{a:1,rm:2}' |
  super -z -c 'over flatten(this) => (
           key[0] != "rm"
           |> yield collect(this)
         )
         |> yield unflatten(this)
  ' -
```

```mdtest-output
{a:1}
```
