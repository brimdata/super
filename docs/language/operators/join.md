### Operator

&emsp; **join** &mdash; combine data from two inputs using a join predicate

### Synopsis

```
<left-input>
| [anti|inner|left|right] join (
  <right-input>
) on <left-key>=<right-key> [[<field>:=]<right-expr>, ...]

( => <left-input> => <right-input> )
| [anti|inner|left|right] join on <left-key>=<right-key> [[<field>:=]<right-expr>, ...]
```

{{% tip "Note" %}}

The first `join` syntax shown above was more recently introduced and is in some
ways similar to other languages such as SQL.  The second was the original `join`
syntax in the language.  Most joins can be expressed using either syntax.  See the
[join tutorial](../../tutorials/join.md)
for details.

{{% /tip %}}

### Description

The `join` operator combines records from two inputs based on whether
the `<left-key>` expression (evaluated in the context of the left input)
is equal to the `<right-key>` expression (evaluated in the context of
the right input) omitting values where there is no match (or including them
in the case of anti join).

The available join types are:
* _inner_ - output only values that match
* _left_ - output all left values with merged components from `<right-expr>`
* _right_ - output as a left join but with the roles of the inputs and `<right-expr>` reversed
* _anti_ - output left values whose left key does not have a matching right key

For anti join, the `<right-expr>` is undefined and thus cannot be specified.

{{% tip "Note" %}}

Currently, only exact equi-join is supported and join keys must be field
expressions. A future version of `join` will have more flexible join
expressions.

{{% /tip %}}

### Examples

The [join tutorial](../../tutorials/join.md) includes several examples.
