# Introduction

SuperDB is a new analytics database that fuses structured and semi-structured data
into a new, unified data model called _super-structured data_.
With super-structured data,
complex problems with modern data stacks become easier to tackle
because relational tables and eclectic JSON data are treated in uniform way
from the ground up.

## Super-structured Data

Super-structured data is strongly typed and self describing.
While compatible with relational schemas, SuperDB does not require such schemas
as they can be modeled as super-structured types.

More specifically, a collection of tuples defined by a statically typed 
record is a _relational table_ while a collection of dynamic but strongly-typed
data can model any sequence of JSON values, e.g., observability data,
application events, system logs, and so forth.

Thus, data in SuperDB is
* strongly typed like databases, but
* dynamically typed like JSON.

Self-describing data makes data easier: when transmitting data from one entity 
to another there is no need for the two sides to agree up front what the schemas
must be in order to communicate and land the data.

## The `super` Command

SuperDB is implemented in a single, standalone executable called `super`.
There are no external dependencies to futz with.
Just [install the binary](getting-started/install.md) and you're off and running.

Like most modern databases that separate compute and storage,
SuperDB is decomposed into a runtime system that may be run directly
on any data inputs like files, streams, or APIs,
and a separate storage layer that rhymes in design with the emergent
[lakehouse pattern](https://www.cidrdb.org/cidr2021/papers/cidr2021_paper17.pdf)
but is based on super-structured data.

The `super` command can execute the SuperDB runtime without a lakehouse:
```
super -c "SELECT 'hello, world'"
```
To interact with a SuperDB lakehouse, the `super db` subcommands and/or
its corresponding API can be utilized.

> Note that the SuperDB lakehouse is still under development and not yet 
> ready for turnkey production use.

## Why Not the Relational Model?

The _en vogue_ argument against a new system like SuperDB is that SQL and the relational 
model (RM) are perfectly good solutions that have stood the test of time 
and there's no need to replace them. 
In fact, 
[a recent paper](https://db.cs.cmu.edu/papers/2024/whatgoesaround-sigmodrec2024.pdf)
from legendary database experts
argues that any attempt to supplant SQL or the RM is doomed to fail
because any good ideas that arise from such efforts will simply be incorporated 
into SQL and the RM.

Yet, the incorporation of the JSON data model into the 
relational model has left much to be desired.  One must basically choose 
between creating columns of "JSON type" that layers in a parallel set of 
operators and behaviors that diverge from core SQL semantics, or
relying upon schema inference to convert JSON into relational tables,
which unfornately does not always work.

For example, suppose this single line of JSON data is in a file called `example.json`:
```json
{"a":[1,"foo"]}
```

> The simple literal `[1,"foo"]` is a contrived example
> but imagine the common design pattern
> of an API returning an array of JSON objects with varying shape
> and you end up with the same mixed-type challenge.

This simple JSON results in unpredictable schema inference.
Clickhouse converts the JSON number `1` to a string:
```sh
$ clickhouse -q "SELECT * FROM 'example.json'"
['1','foo']
```
DuckDB does not do schema infererence at all for the contents 
of the array leaving the elements as type JSON:
```sh
$ duckdb -c "SELECT * FROM 'example.json'"
┌──────────────┐
│      a       │
│    json[]    │
├──────────────┤
│ [1, '"foo"'] │
└──────────────┘
```
And Datafusion simply fails with an error:
```sh
$ datafusion-cli -c "SELECT * FROM 'example.json'" 
DataFusion CLI v46.0.1
Error: Arrow error: Json error: whilst decoding field 'a': expected string got 1
```
It turns out there's no easy way to represent this straightforward
literal array value `[1,'foo']` in these SQLs, e.g., simply including this
value in a SQL expression results in errors:
```sh
$ clickhouse -q "SELECT [1,'foo']"
Code: 386. DB::Exception: There is no supertype for types UInt8, String because some of them are String/FixedString/Enum and some of them are not. (NO_COMMON_TYPE)
$ duckdb -c "SELECT [1,'foo']"
Conversion Error:
Could not convert string 'foo' to INT32

LINE 1: SELECT [1,'foo']
                  ^
$ datafusion-cli -c "SELECT [1,'foo']" 
DataFusion CLI v46.0.1
Error: Arrow error: Cast error: Cannot cast string 'foo' to value of Int64 type
```

The more recent innovation of an open
["variant type"](https://github.com/apache/spark/blob/master/common/variant/README.md)
is more general than JSON but suffers from similar problems.
In both these cases, the JSON type and the variant
type are not individual types but rather entire type systems that differ 
from the base relational type sysetem and so are shoehorned into the relational model
as a parallel type system masquerading as specialized type to make it all work.

Maybe there is a better way?

## Enter Algebraic Types

What's missing here is an easy and native way to represent mixed-type entities.
In modern programming languages, such entities are enabled with a
[sum type or tagged union](https://en.wikipedia.org/wiki/Tagged_union).

While the original conception of the relational data model anticipated 
"product types" --- in fact, describing a relation's schema in terms of
a product type --- it unfortunately did not anticipate sum types.


polymorphic sets...
polymorphic algebra instead of relational algebra

The JSON or variant concepts add dynamic typing inside of the relational model...

how do you achieve dynamic typing and strong typing at the same type?

Static analysis is really important and valuable and a big part of the s
success of the relational model.
How do we turn the dynamic problem into a static problem?

Problems:
* cast
* sql requires static analysis and casts of JSON values
* schema inference 

SuperDB does not propose to replace the RM...

XXX JSON or a variant type...
require casts

SECTION ON POLYMORPHIC

The SuperDB foundations are built on the idea that the databade industry has 
the model inside out.  Instead of putting eclectic data like JSON inside the
static world of a relational column, then inventing all sorts of new functionality
inside of 

DBT jobs taking forever to finish only to discover an error.
SQL++ doesn't have strong types so can't solve the problem.

XXX The pivot

There should be one way of doing things but people love their SQL.
So we solve the GOes-around problem with SuperSQL scoping model:
a separation of SQL operators and pipe operators.

Codd footnote missing sum types

strong typing...
new set of formats with proper sum types
se

XXX clarify db vs super, connect to an instance vs operate directly on inputs

XXX currently no support for connecting to relational systems and open Lake formats,
but that may come...

XXX In a block quote:
XXX ref PRQL, Against SQL, Google Pipes paper, sane QL paper, SQL++

XXX discuss types scaling

TODO: where does this go?
```sh
; duckdb -c "select [1,'foo']"    
Conversion Error:
Could not convert string 'foo' to INT32

LINE 1: select [1,'foo']
                  ^
; clickhouse -q "select [1,'foo']"    
Code: 386. DB::Exception: There is no supertype for types UInt8, String because some of them are String/FixedString/Enum and some of them are not. (NO_COMMON_TYPE)
; datafusion-cli -c "select [1,'foo']"  
DataFusion CLI v46.0.1
Error: Arrow error: Cast error: Cannot cast string 'foo' to value of Int64 type
; ~/demo/zeta/execute_query_macos "select [1,'foo']"
Array elements of types {INT64, STRING} do not have a common supertype
```




## SuperSQL

XXX Challenge of all this is then designing a query language

These two very data styles are treated in the same way with a
unified type system, unified query operators, and unfified storage formats.
There's not a "relational way of doings things" and a different "JSON way of
doing things".

The SuperDB query language is a Pipe SQL adapted super-structured data called _SuperSQL_.
SuperSQL is particularly well suited for data-wrangling use cases like
ETL and data exploration and discovery.  Syntactic shortcuts, keyword search,
and SuperDB Desktop make interactively querying data a breeze.


SuperDB is also good for analytics as a vectorized query runtime built natively
upon on the super-structured data model lies at its foundation.

* x thumbnail about super-structured data
* command... back off on lakehouse
* data model
* supersql
* lakehouse