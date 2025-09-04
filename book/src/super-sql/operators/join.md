### Operator

&emsp; **join** &mdash; combine data from two inputs using a join predicate

### Synopsis

```
<left-input>
| [anti|inner|left|right] join (
  <right-input>
) [as { <left-name>,<right-name> }] [on <predicate> | using ( <field> )]

( <left-input> )
( <right-input> )
| [anti|inner|left|right] join [as { <left-name>,<right-name> }] [on <predicate> | using ( <field> )]

<left-input> cross join ( <right-input> ) [as { <left-name>,<right-name> }]

( <left-input> )
( <right-input> )
| cross join [as { <left-name>,<right-name> }]
```

### Description

The `join` operator combines values from two inputs according to the Boolean-valued
`<predicate>` into two-field records, one field for each side of the join.

Logically, a cross product of all values is formed by taking each
value `L` from `<left-input>` and forming records with all of the values `R` from
the `<right-input>` of the form `{<left-name>:L,<right-name>:R}`.  The result
of the join is the set of all such records that satisfy `<predicate>`.

A _using clause_ may be specified instead of an _on clause_ and
is equivalent to an equi-join predicate of the form:
```
<left-name>.<field> = <right-name>.<field>
```
`<field>` must be an _l-value_, i.e., an expression composed of dot operators and
index operators.

If the _as clause_ is omitted, then `<left-name>` defaults to "left" and
`<right-name>` defaults to "right".

For a _cross join_, neither an _on clause_ or a _using clause_ may be present
and the condition is presumed true for all values so that the
entire cross product is produced.

The output order of the joined values is undefined.

The available join types are:
* _inner_ - as described above
* _left_ - the inner join plus a set of single-field records of the form
`{<left-name>:L}` for each value `L` in `<left-input>` absent from the inner join
* _right_ - the inner join plus a set of single-field records of the form
`{<right-name>:R}` for each value `R` in `<right-input>` absent from the inner join
* _anti_ - the set of records of the form `{<left-name>:L}` for which there is no value
`R` in `<right-input>` where the combined record `{<left-name>:L,<right-name>:R}`
satisfies `<predicate>`
* _cross_ - the entire cross product is computed

As compared to SQL relational scoping, which utilizes table aliases and column aliases
within nested scopes, the pipeline join operator uses dataflow scoping to join data.
Here, all data is combined into joined records that can be operated upon
like any other record without complex scoping logic.

If relational scoping is desired, a SQL [`JOIN`](../sql/join.md) clause
can be used instead.

### Examples

---

_Join some numbers_
```mdtest-spq
# spq
join (values 1,3) on left=right | sort
# input
1
2
3
# expected output
{left:1,right:1}
{left:3,right:3}
```

---

_Join some records with scalar keys_
```mdtest-spq
# spq
join (
    values "foo","baz"
  ) as {recs,key} on key=recs.key
| values recs.value
| sort
# input
{key:"foo",value:1}
{key:"bar",value:2}
{key:"baz",value:3}
# expected output
1
3
```

---
_Join some records via a `using` clause_
```mdtest-spq
# spq
join (
  values {num:1,word:'one'},{num:2,word:'two'}
) using (num)
| {word:right.word, parity:left.parity}
# input
{num:1, parity:"odd"}
{num:2, parity:"even"}
# expected output
{word:"one",parity:"odd"}
{word:"two",parity:"even"}
```

---

_Anti-join some numbers_
```mdtest-spq
# spq
anti join (values 1,3) on left=right | sort
# input
1
2
3
# expected output
{left:2}
```

---

_Cross-product join_
```mdtest-spq
# spq
cross join (values 4,5) as {a,b} | sort
# input
1
2
3
# expected output
{a:1,b:4}
{a:1,b:5}
{a:2,b:4}
{a:2,b:5}
{a:3,b:4}
{a:3,b:5}
```
