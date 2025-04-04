## scopes

> This is a temp doc that will move into Go code comments once we
> agree on the approach.

### need for scopes

We need scoped dataflow so an inner bit of flowgraph can process `this`
under a different name and still reference `this` from the parent scope,
and we need to do this in a nested way.

For example
```
// scope-0 (this comes from batch)
foo := over(bar) => (
  // scope-1 (this comes from scope-0, foo comes from batch)
  yield {a:foo.a,b:this.b} // -> lexical scope: yields into foo (is this confusing?)
)
```
Also, scope can be used to compute temp results that might be used in
a subgraph or just in a large top-level piece of flowgraph.  Currently,
we use a pattern where we `put` temp results into `this`, use them downstream,
then `drop` them.  That's rather silly.
```
// scope-0 (this comes from batch)
with bar := <complex-expression> => (
  // scope-1 (this comes from scope-0, bar comes from expr)
  yield {a:complexFunc(bar,this),b:this.b}
)
```
In the example above, there is no sub-graph traversal but the idea is we can
store a temp result in a named var put in the scope for complex expression.
And it's all recursive...
```
// scope-0 (this comes from batch)
with bar := <complex-expression> => (
  // scope-1 (this comes from scope-0, bar comes from expr)
  yield {a:complexFunc(bar,this),b:this.b}
  | baz := over(a) (
    // scope-2 (this comes from scope-a, bar comes from scope-1, baz comes from batch)
    yield {x:baz,y:bar.a,z:this.x} // yields into baz, is this confusing?
  )
  // Now the value coming out of scope-2 is called `bar`
  yield {inner:bar,foo:f(bar.x,this.y)}
)
```

Perhaps, renaming `this` in nested scopes is too confusing.  We could instead
rename this from the outer scope, which was my first proposal.  This seems more
natual but the syntax for a simple expressions seems less clean:
```
// scope-0 (this comes from batch)
with foo:=this, over(bar) => (
  // scope-1 (foo comes from scope-0, this comes from batch)
  yield {a:this.a,b:foo.b} // -> lexical scope: yields into this (is this less confusing?)
)
```
Or perhaps over should be separated from with:
```
// scope-0 (this comes from batch)
with foo:=this (
  // scope-1 (foo comes from scope-0 this, this comes from batch from over)
  over(bar)
  | yield {a:this.a,b:foo.b} // -> lexical scope: yields into this (is this less confusing?)
)
```
And we could have shorthand for the above:
```
// scope-0 (this comes from batch)
over(bar) with foo:=this (
  yield {a:this.a,b:foo.b} // -> lexical scope: yields into this (is this less confusing?)
)
```

### mechanism

Whatever we decide above, we can solve the scoping problem by carrying
the scope context in a "frame" attached to the batch.  The frame is simply
a slice of super.Values where the slice index can be determined by the compiler
using lexical scope.  Each reference to an identifier can be turned into
a reference to the proper slot in the frame based on lexical scope.  A new
dag.Node will represent such references (dag.FrameRef?).  We also change
the a ref to "this" as a ref to the dataflow val since "this" might refer
to a parent scope value in the frame (dag.FlowVal?)

With this design:
* the dataflow values come from batch.Values
* refs come from batch.Frame
* expr.Eval() is changed to take the dataflow super.Value along with the Frame
* the compiler turns references to "this" or other vars bound by "over" and "with"
into a frame ref or a dataflow ref.

Note we really need to attach the frame to the batch to support concurrency
and wrap all state required by an operator into a single entity.  If we
kept the frame out of the batch then we would just need to change a bunch of
code (like channels that carry proc.Result) to also carry the Frame.

That said, there are also places in the code where we use batch to simply be
an array of values without needing any context.  This makes me think we should
separate these use cases into two interfaces: a dataflow batch and a vanilla value batch.
Then, e.g., the BSUP reader can implement both interfaces and other places in the
code where we just need the vanilla batch, it can be a frameless array of values.

## other approaches

Partiql extends the SQL AS clause on FROM:
```sql
SELECT ...
FROM hr.employeesNest AS e,
     e.projects AS p
```     
In the second AS, there is no table e for e.projects; instead it comes
from the identifier defined just before it.  Quite a hack to SQL.

Asterix SQL++ does the same with different syntax moving the hack out of
FROM and requiring a new keyword UNNEST:
```sql
SELECT u.id AS userId, e.organizationName AS orgName
FROM GleambookUsers u
UNNEST u.employment e
WHERE u.id = 1;
```

Other approaches use UNFLATTEN then treat the result as a table in the
SQL expression.

Morel has this pattern:
```
from e in emps,
    d in depts
  where e.deptno = d.deptno
  yield {e.id, e.deptno, ename = e.name, dname = d.name};
```
It doesn't appear you can nest another from within the outerf rom.

The downside of these approaches is that the un-nesting is carried out
separately from the operations on the unnested results.  With dataflow scopes,
arbitrary logic can be carried out at each level in the nesting since each
nested layer appears explicitly the flowgraph blocks.
