## Containment

The `in` operator has the form
```
<item-expr> in <container-expr>
```
and is true if the `<item-expr>` expression results in a value that
appears somewhere in the `<container-expr>` as an exact match of the item.
The right-hand side value can be any value. For example,
```mdtest-spq
# spq
1 in this
# input
{a:[1,2]}
{b:{c:3}}
{d:{e:1}}
# expected output
{a:[1,2]}
{d:{e:1}}
```

Complex values are recursively traversed to determine if the item is present
anywhere within them:
```mdtest-spq
# spq
{s:"foo"} in this
# input
{s:"foo"}
{s:"foo",t:"bar"}
{a:{s:"foo"}}
[1,{s:"foo"},2]
# expected output
{s:"foo"}
{a:{s:"foo"}}
[1,{s:"foo"},2]
```

You can also use this operator with a static array:
```mdtest-spq
# spq
unnest accounts | where id in [1,2]
# input
{accounts:[{id:1},{id:2},{id:3}]}
# expected output
{id:1}
{id:2}
```
