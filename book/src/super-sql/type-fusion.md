## Type Fusion

(https://openproceedings.org/2017/conf/edbt/paper-62.pdf)

_Type fusion_ is a process by which a set of input types is merged together 
to form one output type where all values of the input types are subtypes
of the output type such that any value of an input type is respresentable in 
a _reasonable way_ (e.g., by inserting null values) with an equivalent value
in the output type.

When the output type models a relational schema and the input types are derived
from semi-structured data is a target of the schema, then this technique resembles
_schema inference_ in other systems.

> _Schema inference also involves the inference of particular primitive data types from
> string data when the strings represent dates, times, IP addresses, etc.
> This step is orthogonal to type fusion and can be applied to the input 
> types of any type fusion algorithm._

A fused type computed over heterogeneous data represents a typical
design pattern of a data warehouse, where a single very-wide type-fused schema
defines slots for all possible input values and the columns are
sparsely populated by each row value as the missing columns are set to null.

While super-structured data natively represents heterogeou data and
fortunately does not require a fused schema to persist data, type fusion
is nonetheless very useful:
* for _data exploration_, when sampling or filtering data to look at
slices of raw data that are fused together;
* for _exporting super-structured data_ to other systems and formats,
where formats like Parquet or a tabular structure like CSV require fixed schemas; and,
* for _ETL_, where data might be gathered from APIs using SuperDB,
transformed in a SuperDB pipeline, and written to another data warehouse.

Unfortunately, when data leaves a super-structured format using
type fusion to accomplish this, the original data must be altered
to fit into the rigid structure of these output formats.

===

This operation is often in other database systems,
but in SuperSQL, the technique is based purely on types so the preferred
term here is
[_type fusion_].


### Basic Mechanism

XXX
The merged type is constructed intelligently in the sense that type
`{a:string}` and `{b:string}` is fused into type `{a:string,b:string}`
instead of the union type `{a:string}|{b:string}`.

> TBD: document the algorithm here in more detail.
> The operator takes no paramters but we are experimenting with ways to
> control how field with the same name but different types are merged
> especially in light of complex types like arrays, sets, and so forth.
