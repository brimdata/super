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

```mdtest-spq {data-layout="stacked"}
# spq
yield {typeof:typeof(this),typeunder:typeunder(this)}
# input
{which:"chocolate"}(=flavor)
# expected output
{typeof:<flavor={which:string}>,typeunder:<{which:string}>}
```
