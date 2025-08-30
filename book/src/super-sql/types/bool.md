### Booleans

The `bool` type represents a type that has the values `true`, `false`,
or `null`.

#### Examples

---

_Comparisons produces Boolean values_

```mdtest-spq
# spq
values 1==1, 1>2
# input
null
# expected output
true
false
```

---
_Booleans are used as predicates_

```mdtest-spq
# spq
values 1==1, 1>2
# input
null
# expected output
true
false
```

---
_Booleans operators perform logic on Booeleans_

```mdtest-spq
# spq
values {and:a and b, or:a or b}
# input
{a:false,b:false}
{a:false,b:true}
{a:true,b:true}
# expected output
{and:false,or:false}
{and:false,or:true}
{and:true,or:true}
```

---

_Boolean aggregate functions_

```mdtest-spq {data-layout="stacked"}
# spq
aggregate andA:=and(a), andB:=and(b), orA:=or(a), orB:=or(b)
# input
{a:false,b:false}
{a:false,b:true}
# expected output
{andA:false,andB:false,orA:false,orB:true}
```