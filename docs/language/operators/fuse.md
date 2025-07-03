### Operator

&emsp; **fuse** &mdash; coerce all input values into a merged type

### Synopsis

```
fuse
```
### Description

The `fuse` operator reads all of its input, computes an "intelligent merge"
of varied types in the input, then adjusts each output value
to conform to the merged type.

The merged type is constructed intelligently in the sense that type
`{a:string}` and `{b:string}` is fused into type `{a:string,b:string}`
instead of the union type `({a:string},{b:string})`.

> TBD: document the algorithm here in more detail.
> The operator takes no paramters but we are experimenting with ways to
> control how field with the same name but different types are merged
> especially in light of complex types like arrays, sets, and so forth.

Because all values of the input must be read to compute the union,
`fuse` may spill its input to disk when memory limits are exceeded.

`Fuse` is not normally needed for super-structured data as the super data model supports
heterogenous sequences of values.  However, `fuse` can be quite useful
during data exploration when sampling or filtering data to look at
slices of raw data that are fused together.  `Fuse` is also useful for
transforming arbitrary super data to prepare it for formats that require
a uniform schema like Parquet or a tabular structure like CSV.
Unfortunately, when data leaves a super-structured format using `fuse` to accomplish this,
the original data must be altered to fit into the rigid structure of
these output formats.

A fused type over many heterogeneous values also represents a typical
design pattern of a data warehouse where a single very-wide schema
defines slots for all possible input values where the columns are
sparsely populated by each row value as the missing columns are set to null.
Super data is super-structured, and fortunately, does not require such a structure.

### Examples

_Fuse two records_
```mdtest-spq
# spq
fuse
# input
{a:1}
{b:2}
# expected output
{a:1,b:null(int64)}
{a:null(int64),b:2}
```

_Fuse records with type variation_
```mdtest-spq
# spq
fuse
# input
{a:1}
{a:"foo"}
# expected output
{a:1(int64|string)}
{a:"foo"(int64|string)}
```

_Fuse records with complex type variation_
```mdtest-spq {data-layout="stacked"}
# spq
fuse
# input
{a:[1,2]}
{a:["foo","bar"],b:10.0.0.1}
# expected output
{a:[1,2]([int64|string]),b:null(ip)}
{a:["foo","bar"]([int64|string]),b:10.0.0.1}
```

_The table format clarifies what fuse does_
```mdtest-command
echo '{a:1}{b:2}{a:3}' | super -f table -c fuse -
```
=>
```mdtest-output
a b
1 -
- 2
3 -
```
