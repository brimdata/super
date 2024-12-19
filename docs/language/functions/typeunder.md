### Function

&emsp; **typeunder** &mdash; the underlying type of a value

### Synopsis

```
typeunder(val: any) -> type
```

### Description

The _typeunder_ function returns the type of its argument `val`.  If this type is a
[named type](../../formats/zed.md#3-named-type), then the referenced type is
returned instead of the named type.

### Examples

```mdtest-command
echo  '{which:"chocolate"}(=flavor)' |
  super -z -c 'yield {typeof:typeof(this),typeunder:typeunder(this)}' -
```

```mdtest-output
{typeof:<flavor={which:string}>,typeunder:<{which:string}>}
```
