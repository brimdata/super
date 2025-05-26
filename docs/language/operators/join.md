### Operator

&emsp; **join** &mdash; combine data from two inputs using a join predicate

### Synopsis

```
<left-input>
| [anti|inner|left|right] join [as { <left-name>,<right-name> }] (
  <right-input>
) [on <predicate>]

( => <left-input> => <right-input> )
| [anti|inner|left|right] join [as { <left-name>,<right-name> }] [on <predicate>]

==========
DEPRECATED
==========
( => <left-input> => <right-input> )
| [anti|inner|left|right] join on <left-key>=<right-key> [[<field>:=]<right-expr>, ...]
```

{{% tip "Note" %}}

The first syntax should be used as support for the syntax marked DEPRECATED will be
removed at some point.

{{% /tip %}}

### Description

The `join` operator combines values from two inputs according to the Boolean-valued
`<predicate>`.  Logically, a cross product of all values is formed by taking each
value `L` from `<left-input>` and forming records with all of the values `R` from
the `<right-input>` of the form `{<left-name>:L,<right-name>:R}`.  The result
of the join is the set of all such records that satisfy `<predicate>`.

The output order of the resulting records is undefined.

If the "as clause" is ommited, then `<left-name>` defaults to "left" and
`<right-name>` defaults to "right".

If `<predicate>` is ommited, then it is presumed true and the entire cross
product is produced.

The available join types are:
* _inner_ - as described above
* _left_ - the inner join plus a set of single-field records of the form
`{<left-name>:L}` for each value `L` in `<left-input>` absent from the inner join
* _right_ - the inner join plus a set of single-field records of the form
`{<right-name>:R}` for each value `R` in `<right-input>` absent from the inner join
* _anti_ - the set of records of the form `{<left-name>:L}` for which there is no value
`R` in `<right-input>` where the combined record `{<left-name>:L,<right-name>:R}`
satisfies `<predicate>`

### Examples

_Join some numbers_
```mdtest-spq-notyet
# spq
join (from (yield 1,3)) on left=right | sort
# input
1
2
3
# expected output
{left:1,right:1}
{left:3,right:3}
```

_Join some records with scalar keys_
```mdtest-spq-notyet
# spq
join as {recs,key} (from (yield "foo","baz")) on key=recs.key | yield recs.value | sort
# input
{key:"foo",value:1}
{key:"bar",value:2}
{key:"baz",value:3}
# expected output
1
3
```

_Join some records requiring a cross-product calculation_
```mdtest-spq-notyet
# spq
join as {b,a} (from (yield {id:"apple"},{id:"chair"},{id:"car"})) on grep("a", a.id) and grep("b", b.key) | sort
# input
{key:"foo",value:1}
{key:"bar",value:2}
{key:"baz",value:3}
# expected output
{b:{key:"bar",value:2},a:{id:"apple"}}
{b:{key:"bar",value:2},a:{id:"car"}}
{b:{key:"baz",value:3},a:{id:"apple"}}
{b:{key:"baz",value:3},a:{id:"car"}}
```

_Anti-join some numbers_
```mdtest-spq-notyet
# spq
anti join (from (yield 1,3)) on left=right | sort
# input
1
2
3
# expected output
{left:2,right:2}
```

The [join tutorial](../../tutorials/join.md) includes several more examples.
