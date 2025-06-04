## Functions

New functions are declared with the syntax
```
fn <id> ( [<param> [, <param> ...]] ) : <expr>
```
where
* `<id>` is an identifier representing the name of the function,
* each `<param>` is an identifier representing a positional argument to the function, and
* `<expr>` is any [expression](../expressions/intro.md) that implements the function.

Function declarations must appear in the declaration section of a [scope](../syntax.md#scope).

The function body `<expr>` may refer the passed-in arguments by name.

Specifically, the references to the named parameters are
field references of the special value `this`, as in any expression.
In particular, the value of `this` referenced in a function body
is formed as record from the actual values passed to the function
where the field names correspond to the parameters of the function.

For example, the function `add` as defined by
```
fn add(a,b): a+b
```
when invoked as
```
values {x:1} | values add(x,1)
```
is passed the record the `{a:x,b:1}`, which after resolving `x` to `1`,
is `{a:1,b:1}` and thus evaluates the expression
```
this.a + this.b
```
which results in `2`.

Any function-as-value arguments passed to a function do not appear in the `this`
record formed from the parameters.  Instead, function values are expanded at their
call sites in a macro-like fashion.

Functions may be recursive.  If the maximum call stack depth is exceeded,
the function returns an error value indicating so.  Recursive functions that
run for an extended period of time without exceeding the stack depth will simply
be allowed to run indefinitely and stall the query result.

### Subquery Functions

Since the body of a function is any expression and an expression may be
a subquery, function bodies can be defined as [subqueries](../expressions/subqueries.md).
This leads to the commonly used pattern of a subquery function:
```
fn <id> ( [<param> [, <param> ...]] ) : (
    <query>
)
```
where `<query>` is any [query](../syntax.md) and is simply wrapped in parentheses
to form the subquery.

As with any subquery, when multiple results are expected, an array subquery
may be used by wrapping `<query>` in square brackets instead of parentheses:
```
fn <id> ( [<param> [, <param> ...]] ) : [
    <query>
]
```

### Recursive Subqueries

When subqueries are combined with recursive invocation of the function they
appear in, some powerful patterns can be constructed.

For example, the [visitor-walk pattern](https://en.wikipedia.org/wiki/Visitor_pattern)
can be implemented using recursive subqueries and function values.

Here's a template for walk:
```
fn walk(node, visit):
  case kind(node)
  when "array" then
    [unnest node | walk(this, visit)]
  when "record" then
    unflatten([unnest flatten(node) | {key,value:walk(value, visit)}])
  when "union" then
    walk(under(node), visit)
  else visit(node)
  end
```
> _Note in this case, we are traversing only records and arrays.  Support for flattening
> and unflattening maps and sets is forthcoming._

Here, `walk` is invoking an [array subquery](../expressions/subqueries.md) on the unnested
entities (records or arrays), calling the `walk` function recursively on each item,
then assembling the results back into an array (i.e., the raw result of the array subquery)
or a record (i.e., calling unflatten on the key/value pairs returned in the array).

If we call `walk` with this function on an arbitrary nested value
```
fn addOne(node): case typeof(node) when <int64> then node+1 else node end
```
then each leaf value of the nested value of type `int64` would be incremented
while the other leaves would be left alone.  See the example below.


### Examples

---

_A simple function that adds two numbers_

```mdtest-spq
# spq
fn add(a,b): a+b
values add(x,y)
# input
{x:1,y:2}
{x:2,y:2}
{x:3,y:3}
# expected output
3
4
6
```

---

_A simple recursive function_

```mdtest-spq
# spq
fn fact(n): n<=1 ? 1 : n*fact(n-1)
values fact(5)
# input
null
# expected output
120
```
---
_A subquery function that computes some stats over numeric arrays_

```mdtest-spq
# spq
fn stats(numbers): (
    unnest numbers
    | sort this
    | avg(this),min(this),max(this),mode:=collect(this)
    | mode:=mode[(len(mode)/2)+1]
) 
values stats(a)
# input
{a:[3,1,2]}
{a:[4]}
# expected output
{avg:2.,min:1,max:3,mode:2}
{avg:4.,min:4,max:4,mode:4}
```
---
_Function arguments are actually fields in the "this" record_

```mdtest-spq
# spq
fn that(a,b,c): this
values that(x,y,3)
# input
{x:1,y:2}
# expected output
{a:1,b:2,c:3}
```
---
_Functions passed as values do not appear in the "this" record_

```mdtest-spq
# spq
fn apply(f,arg):{that:this,result:f(arg)}
fn square(x):x*x
values apply(&square,val)
# input
{val:1}
{val:2}
# expected output
{that:{arg:1},result:1}
{that:{arg:2},result:4}
```

---
_Recursive subqueries inside function implementing walk-visitor pattern_

```mdtest-spq
# spq
fn walk(node, visit):
  case kind(node)
  when "array" then
    [unnest node | walk(this, visit)]
  when "record" then
    unflatten([unnest flatten(node) | {key,value:walk(value, visit)}])
  when "union" then
    walk(under(node), visit)
  else visit(node)
  end
fn addOne(node): case typeof(node) when <int64> then node+1 else node end
values walk(this, &addOne)
# input
1
[1,2,3]
[{x:[1,"foo"]},{y:2}]
# expected output
2
[2,3,4]
[{x:[2,"foo"]},{y:3}]
```
