# The Zed Data Model

> TL;DR The Zed data model defines a new and easy way to manage, store,
> and process data utilizing an emerging concept called
[super-structured data](#2-zed-a-super-structured-pattern).
> The [data model specification](zed.md) defines the high-level model that is realized
> in a [family of interoperable serialization formats](#3-the-data-model-and-formats),
> providing a unified approach to row, columnar, and human-readable formats.
> Zed is a superset of both the dataframe/table model of relational systems and the
> semi-structured model that is used ubiquitously in development as JSON and by NOSQL
> data stores.

The Specifications

* [Zed Data Model Spec](zed.md)
* [ZSON Spec](zson.md)
* [ZNG Spec](zng.md)
* [ZST Spec](zst.md)
* [ZJSON Spec](zjson.md)

---

Table of Contents

* [1. Background](#1-background)
  + [1.1 The Tabular-structured Pattern](#11-the-tabular-structured-pattern)
  + [1.2 The Semi-structured Pattern](#12-the-semi-structured-pattern)
  + [1.3 The Hybrid Pattern](#13-the-hybrid-pattern)
* [2. Zed: A Super-structured Pattern](#2-zed-a-super-structured-pattern)
  + [2.1 Zed and Schemas](#21-zed-and-schemas)
  + [2.2 Type Combinatorics](#22-type-combinatorics)
  + [2.3 Analytics Performance](#23-analytics-performnce)
  + [2.4 First-class Types](#24-first-class-types)
  + [2.5 First-class Errors](#25-first-class-errors)
* [3. The Data Model and Formats](#3-the-data-model-and-formats)
* [4. Examples](#4-examples)

---

## 1. Background

Zed offers a new and improved way to think about and manage data.

Modern data models are typically described in terms of their _structured-ness_:
* _tabular-structured_, often simply called _"structured"_,
where a specific schema is defined to describe a table and values are enumerated that conform to that schema;
* _semi-structured_, where arbitrarily complex, hierarchical data structures
define the data and values do not fit neatly into tables, e.g., JSON and XML; and
* _unstructured_, where arbitrary text is formatted in accordance with
external, often vague, rules for its interpretation.

### 1.1 The Tabular-structured Pattern

CSV is arguably the simplest but most frustrating format that follows the tabular-structured
pattern.  It provides a bare bones schema consisting of the names of the columns as the
first line of a file followed by a list of comma-separated, textual values
whose types must be inferred from the text.  The lack of a universally adopted
specification for CSV is an all too common source of confusion and frustration.

The traditional relational database, on the other hand,
offers the classic, comprehensive example of the tabular-structured pattern.
The table columns have precise names and types.
Yet, like CSV, there is no universal standard format for relational tables.
The [_SQLite file format_](https://sqlite.org/fileformat.html)
is arguably the _de facto_ standard for relational data,
but this format describes a whole, specific database --- indexes and all ---
rather than a stand-alone table.

Instead, file formats like Avro, ORC, and Parquet arose to represent tabular data
with an explicit schema followed by a sequence of values that conform to the schema.
While Avro and Parquet schemas can also represent semi-structured data, all of the
values in a given Avro or Parquet file must conform to the same schema.
The [Iceberg specification](https://iceberg.apache.org/#spec/)
defines data types and metadata schemas for how large relational tables can be
managed as a collection of Avro, ORC, and/or Parquet files.

### 1.2 The Semi-structured Pattern

JSON, on the other hand, is the ubiquitous example of the semi-structured pattern.
Each JSON value is self-describing in terms of its
structure and types, though the JSON type system is limited.

When a sequence of JSON objects is organized into a stream
(perhaps [separated by newlines](http://ndjson.org/))
each value can take on any form.
When all the values have the same form, the JSON sequence
begins to look like a relational table, but the lack of a comprehensive type system,
a union type, and precise semantics for columnar layout limits this interpretation.

[BSON](https://bsonspec.org/)
and [Ion](https://amzn.github.io/ion-docs/)
were created to provide a type-rich elaboration of the
semi-structured model of JSON along with performant binary representations
though there is no mechanism for precisely representing the type of
a complex value like an object or an array other than calling it
type "object" or type "array", e.g., as compared to "object with field s
of type string" or "array of number".

[JSON Schema](https://json-schema.org/)
addresses JSON's lack of schemas with an approach to augment
one or more JSON values with a schema definition itself expressed in JSON.
This creates a parallel type system for JSON, which is useful and powerful in many
contexts, but introduces schema-management complexity when simply trying to represent
data in its natural form.

### 1.3 The Hybrid Pattern

As the utility and ease of the semi-structured design pattern emerged,
relational system design, originally constrained by the tabular-structured
design pattern, has embraced the semi-structured design pattern
by adding support for semi-structured table columns.
"Just put JSON in a column."

[SQL++](https://asterixdb.apache.org/docs/0.9.7.1/sqlpp/manual.html)
pioneered the extension of SQL to semi-structured data by
adding support for referencing and unwinding complex, semi-structured values,
and most modern SQL query engines have adopted variations of this model
and have extended the relational model with a semi-structured column type.

But once you have put a number of columns of JSON data into a relational
table, is it still appropriately called "structured"?
Instead, we call this approach the hybrid tabular-/semi-structured pattern,
or more simply, _"the hybrid pattern"_.

## 2. Zed: A Super-structured Pattern

The insight in Zed is to remove the tabular and schema concepts from
the underlying data model altogether and replace them with a granular and
modern type system inspired by general-purpose programming languages.
Instead of defining a single, composite schema to
which all values must conform, the Zed type system allows each value to freely
express its type in accordance with the type system.

In this approach,
Zed is neither tabular nor semi-structured.  Zed is "super-structured".

In particular, the Zed record type looks like a schema but when
serializing Zed data, the model is very different.  A Zed sequence does not
comprise a record-type declaration followed by a sequence of
homogeneously-typed record values, but instead,
is a sequence of arbitrarily typed Zed values, which may or may not all
be records.

Yet when a sequence of Zed values _in fact conforms to a uniform record type_,
then such a collection of Zed records looks precisely like a relational table.
Here, the record type
of such a collection corresponds to a well-defined schema consisting
of field names (i.e, column names) where each field has a specific Zed type.
Zed also has named types, so by simply naming a particular record type
(i.e., a schema), a relational table can be projected from a pool of Zed data
with a simple type query for that named type.

But unlike traditional relational tables, these Zed-constructed tables can have arbitrary
structure in each column as Zed allows the fields of a record
to have an arbitrary type.  This is very different compared to the hybrid pattern:
all Zed data at all levels conforms to the same data model.  Here, both the
tabular-structured and semi-structured patterns are representable in a single model.
Unlike the hybrid pattern, systems based on Zed have
no need to simultaneously support two very different data models.

In other words, Zed unifies the relational data model of SQL tables
with the document model of JSON into a _super-structured_
design pattern enabled by the Zed type system.
An explicit, uniquely-defined type of each value precisely
defines its entire structure, i.e., its super-structure.  There is
no need to traverse each hierarchical value --- as with JSON, BSON, or Ion ---
to discover each value's structure.

And because Zed derives it design from the vast landscape
of existing formats and data models, it was deliberately designed to be
a superset of --- and thus interoperable with --- a broad range of formats
including JSON, BSON, Ion, Avro, ORC, Parquet, CSV, JSON Schema, and XML.

As an example, most systems that are based on semi-structured data would
say the JSON value
```
{"a":[1,"foo"]}
```
is of type object and the value of key `a` is type array.
In Zed, however, this value's type is type `record` with field `a`
of type `array` of type `union` of `int64` and `string`,
expressed succinctly in ZSON as
```
{a:[(int64,string)]}
```
This is super-structuredness in a nutshell.

### 2.1 Zed and Schemas

While the Zed data model removes the schema constraint,
the implication here is not that schemas are unimportant;
to the contrary, schemas are foundational.  Schemas not only define agreement
and semantics between communicating entities, but also serve as the cornerstone
for organizing and modeling data for data engineering and business intelligence.

That said, schemas often create complexity in system designs
where components might simply want to store and communicate data in some
meaningful way.  For example, an ETL pipeline should not break when upstream
structural changes prevent data from fitting in downstream relational tables.
Instead, the pipeline should continue to operate and the data should continue
to land on the target system without having to fit into a predefined table,
while also preserving its super-structure.

This is precisely what Zed enables.  A system layer above and outside
the scope of the Zed data layer can decide how to adapt to the structural
changes with or without administrative intervention.

To this end, whether all the values must conform to a schema and
how schemas are managed, revised, and enforced is all outside the scope of Zed;
rather, the Zed data model provides a flexible and rich foundation
for schema interpretation and management.

### 2.2 Type Combinatorics

A common objection to using a type system to represent schemas is that
xdiverse applications generating arbitrarily structured data can produce
a combinatoric explosion of types for each shape of data.

In practice, this condition rarely arises.  Applications generating
"arbitrary" JSON data generally conform to a well-defined set of
JSON object structures.

A few rare applications carry unique data values as JSON object keys,
though this is considered bad practice.

Even so, this is all manageable in the Zed data model as types are localized
in scope.  The number of types that must be defined in a stream of values
is linear in the input size.  Since data is self-describing and there is
no need for a global schema registry in Zed, this hypothetical problem is moot.

### 2.3 Analytics Performance

One might think that removing schemas from the Zed data model would conflict
with an efficient columnar format for Zed, which is critical for
high-performance analytics.
After all, database
tables and formats like Parquet and ORC all require schemas to organize values
and then rely upon the natural mapping of schemas to columns.

Super-structure, on the other hand, provides an alternative approach to columnar structure.
Instead of defining a schema and then fitting a sequence of values into their appropriate
columns based on the schema, Zed values self-organize into columns based on their
super-structure.  Here columns are created dynamically as data is analyzed
and each top-level type induces a specific set of columns.  When all of the
values have the same top-level type (i.e., like a schema), then the Zed columnar
object is just as performant as a traditional schema-based columnar format like Parquet.

### 2.4 First-class Types

With first-class types, any type can also be a value, which means that in
a properly designed query and analytics system based on Zed, a type can appear
anywhere that a value can appear.  In particular, types can be aggregation keys.

This is very powerful for data discovery and introspection.  For example,
to count the different shapes of data, you might have a SQL-like query,
operating on each input value as `this`, that has the form:
```
  SELECT count(), typeof(this) as shape GROUP by shape, count
```
Likewise, you could select a sample value of each shape like this:
```
  SELECT shape FROM (
    SELECT any(this) as sample, typeof(this) as shape GROUP by shape,sample
  )
```
The Zed language is exploring syntax so that such operations are tighter
and more natural given the super-structure of Zed.  For example, the above
two SQL-like queries could be written as:
```
  count() by shape:=typeof(this)
  any(this) by shape:=typeof(this) | cut shape
```

### 2.5 First-class Errors

In SQL based systems, errors typically
result in cryptic messages or null values offering little insight as to the
actual cause of the error.

Zed however includes first-class errors.  When combined with the super-structured
data model, error values may appear anywhere in the output and operators
can propagate or easily wrap errors so complicated analytics pipelines
can be debugged by observing the location of errors in the output results.

## 3. The Data Model and Formats

The concept of super-structured data and first-class types and errors
is solidified in the [specification of the Zed data model](zed.md),
which defines the model but not the serialization formats.

A set of companion documents define a family of tightly integrated
serialization formats that all adhere to the same Zed data model,
providing a unified approach to row, columnar, and human-readable formats:

* [ZSON](zson.md) is a JSON-like, human readable format for Zed data.  All JSON
documents are Zed values as the ZSON format is a strict superset of the JSON syntax.
* [ZNG](zng.md) is a row-based, binary representation of Zed data somewhat like
Avro but with Zed's more general model to represent a sequence of arbitrarily-typed
values.
* [ZST](zst.md) is a columnar version of ZNG like Parquet or ORC but also
embodies Zed's more general model for hetereogeneous and self-describing schemas.
* [Zed over JSON](zjson.md) defines a JSON format for encapsulating Zed data
in JSON for easy decoding by JSON-based clients, e.g.,
the [Zealot JavaScript library](https://github.com/brimdata/zealot)
and the [Zed Python library](../../python/zed).

Because all of the formats conform to the same Zed data model, conversions between
a human-readable form, a row-based binary form, and a row-based columnar form can
be trivially carried out with no loss of information.  This is the best of both worlds:
the same data can be easily expressed in and converted between a human-friendly
and easy-to-program text form alongside efficient row and columnar formats.

## 4. Examples

To motivate the details of the Zed data model, a few examples are provided below
using the human-readable ZSON format to express Zed values.

The simplest Zed value is a single value, perhaps a string like this:
```
"hello, world"
```
There's no need for a type declaration here.  It's explicitly a string.

A relational table might look like this:
```
{ city: "Berkeley", state: "CA", population: 121643 (uint32) } (=city_schema)
{ city: "Broad Cove", state: "ME", population: 806 (uint32) } (=city_schema)
{ city: "Baton Rouge", state: "LA", population: 221599 (uint32) } (=city_schema)
```
This ZSON text here depicts three record values.  It defines a type called `city_schema`
and the inferred type of the `city_schema` has the signature:
```
{ city:string, state:string, population:uint32 }
```
When all the values in a sequence have the same record type, the sequence
can be interpreted as a _table_, where the ZSON record values form the _rows_
and the fields of the records form the _columns_.  In this way, these
three records form a relational table conforming to the schema `city_schema`.

In contrast, a ZSON text representing a semi-structured sequence of log lines
might look like this:
```
{
    info: "Connection Example",
    src: { addr: 10.1.1.2, port: 80 (uint16) } (=socket),
    dst: { addr: 10.0.1.2, port: 20130 (uint16) } (=socket)
} (=conn)
{
    info: "Connection Example 2",
    src: { addr: 10.1.1.8, port: 80 (uint16) } (=socket),
    dst: { addr: 10.1.2.88, port: 19801 (uint16) } (=socket)
} (=conn)
{
    info: "Access List Example",
    nets: [ 10.1.1.0/24, 10.1.2.0/24 ]
} (=access_list)
{ metric: "A", ts: 2020-11-24T08:44:09.586441-08:00, value: 120 }
{ metric: "B", ts: 2020-11-24T08:44:20.726057-08:00, value: 0.86 }
{ metric: "A", ts: 2020-11-24T08:44:32.201458-08:00, value: 126 }
{ metric: "C", ts: 2020-11-24T08:44:43.547506-08:00, value: { x:10, y:101 } }
```
In this case, the first records defines not just a record type
called `conn`, but also a second embedded record type called `socket`.
The parenthesized decorators are used where a type is not inferred from
the value itself:
* `socket` is a record with typed fields `addr` and `port` where `port` is an unsigned 16-bit integer, and
* `conn` is a record with typed fields `info`, `src`, and `dst`.

The subsequent value defines a type called `access_list`.  In this case,
the `nets` field is an array of networks and illustrates the helpful range of
primitive types in ZSON.  Note that the syntax here implies
the type of the array, as it is inferred from the type of the elements.

Finally, there are four more values that show ZSON's efficacy for
representing metrics.  Here, there are no type decorators as all of the field
types are implied by their syntax, and hence, the top-level record type is implied.
For instance, the `ts` field is an RFC 3339 date and time string,
unambiguously the primitive type `time`.  Further,
note that the `value` field takes on different types and even a complex record
type on the last line.  In this case, there is a different type top-level
record type implied by each of the three variations of type of the `value` field.
