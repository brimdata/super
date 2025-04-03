---
weight: 4
title: Super Columnar
heading: Super Columnar Specification
---

Super Columnar is a file format based on
the [super data model](data-model.md) where data is stacked to form columns.
Its purpose is to provide for efficient analytics and search over
bounded-length sequences of [super-structured data](./_index.md#2-a-super-structured-pattern) that is stored in columnar form.

Like [Parquet](https://github.com/apache/parquet-format),
Super Columnar provides an efficient representation for semi-structured data,
but unlike Parquet, Super Columnar is not based on schemas and does not require
a schema to be declared when writing data to a file.  Instead,
it exploits the nature of super-structured data: columns of data
self-organize around their type structure.

## Super Columnar Files

A Super Columnar file encodes a bounded, ordered sequence of values.
To provide for efficient access to subsets of Super Columnar-encoded data (e.g., columns),
the file is presumed to be accessible via random access
(e.g., range requests to a cloud object store or seeks in a Unix file system)
and is therefore not intended as a streaming or communication format.

A Super Columnar file can be stored entirely as one storage object
or split across separate objects that are treated
together as a single Super Columnar entity.  While the format provides much flexibility
for how data is laid out, it is left to an implementation to lay out data
in intelligent ways for efficient sequential read accesses of related data.

## Column Streams

The Super Columnar data abstraction is built around a collection of _column streams_.

There is one column stream for each top-level type encountered in the input where
each column stream is encoded according to its type.  For top-level complex types,
the embedded elements are encoded recursively in additional column streams
as described below.  For example,
a [record column](#record-column) encodes a [presence column](#presence-columns) encoding any null value for
each field then encodes each non-null field recursively, whereas
an [array column](#array-column) encodes a sequence of "lengths" and encodes each
element recursively.

Values are reconstructed one by one from the column streams by picking values
from each appropriate column stream based on the type structure of the value and
its relationship to the various column streams.  For hierarchical records
(i.e., records inside of records, or records inside of arrays inside of records, etc.),
the reconstruction process is recursive (as described below).

## The Physical Layout

The overall layout of a Super Columnar file is comprised of the following sections,
in this order:
* the data section,
* the reassembly section, and
* the trailer.

This layout allows an implementation to buffer metadata in
memory while writing column data in a natural order to the
data section (based on the volume statistics of each column),
then write the metadata into the reassembly section along with the trailer
at the end.  This allows a stream to be converted to a Super Columnar file
in a single pass.

{{% tip "Note" %}}

That said, the layout is
flexible enough that an implementation may optimize the data layout with
additional passes or by writing the output to multiple files then
merging them together (or even leaving the Super Columnar entity as separate files).

{{% /tip %}}

### The Data Section

The data section contains raw data values organized into _segments_,
where a segment is a seek offset and byte length relative to the
data section.  Each segment contains a sequence of
[primitive-type values](data-model.md#1-primitive-types),
encoded as counted-length byte sequences where the counted-length is
variable-length encoded as in the [Super Binary specification](bsup.md).
Segments may be compressed.

There is no information in the data section for how segments relate
to one another or how they are reconstructed into columns.  They are just
blobs of Super Binary data.

{{% tip "Note" %}}

Unlike Parquet, there is no explicit arrangement of the column chunks into
row groups but rather they are allowed to grow at different rates so a
high-volume column might be comprised of many segments while a low-volume
column must just be one or several.  This allows scans of low-volume record types
(the "mice") to perform well amongst high-volume record types (the "elephants"),
i.e., there are not a bunch of seeks with tiny reads of mice data interspersed
throughout the elephants.

{{% /tip %}}

{{% tip "TBD" %}}

The mice/elephants model creates an interesting and challenging layout
problem.  If you let the row indexes get too far apart (call this "skew"), then
you have to buffer very large amounts of data to keep the column data aligned.
This is the point of row groups in Parquet, but the model here is to leave it
up to the implementation to do layout as it sees fit.  You can also fall back
to doing lots of seeks and that might work perfectly fine when using SSDs but
this also creates interesting optimization problems when sequential reads work
a lot better.  There could be a scan optimizer that lays out how the data is
read that lives under the column stream reader.  Also, you can make tradeoffs:
if you use lots of buffering on ingest, you can write the mice in front of the
elephants so the read path requires less buffering to align columns.  Or you can
do two passes where you store segments in separate files then merge them at close
according to an optimization plan.

{{% /tip %}}

### The Reassembly Section

The reassembly section provides the information needed to reconstruct
column streams from segments, and in turn, to reconstruct the original values
from column streams, i.e., to map columns back to composite values.

{{% tip "Note" %}}

Of course, the reassembly section also provides the ability to extract just subsets of columns
to be read and searched efficiently without ever needing to reconstruct
the original rows.  How well this performs is up to any particular
Super Columnar implementation.

Also, the reassembly section is in general vastly smaller than the data section
so the goal here isn't to express information in cute and obscure compact forms
but rather to represent data in an easy-to-digest, programmer-friendly form that
leverages Super Binary.

{{% /tip %}}

The reassembly section is a Super Binary stream.  Unlike Parquet,
which uses an externally described schema
(via [Thrift](https://thrift.apache.org/)) to describe
analogous data structures, we simply reuse Super Binary here.

#### The Super Types

This reassembly stream encodes 2*N+1 values, where N is equal to the number
of top-level types that are present in the encoded input.
To simplify terminology, we call a top-level type a "super type",
e.g., there are N unique super types encoded in the Super Columnar file.

These N super types are defined by the first N values of the reassembly stream
and are encoded as a null value of the indicated super type.
A super type's integer position in this sequence defines its identifier
encoded in the [super column](#the-super-column).  This identifier is called
the super ID.

{{% tip "Note" %}}

Change the first N values to type values instead of nulls?

{{% /tip %}}

The next N+1 records contain reassembly information for each of the N super types
where each record defines the column streams needed to reconstruct the original
values.

#### Segment Maps

The foundation of column reconstruction is based on _segment maps_.
A segment map is a list of the segments from the data area that are
concatenated to form the data for a column stream.

Each segment map that appears within the reassembly records is represented
with an array of records that represent seek ranges conforming to this
type signature:
```
[{offset:uint64,length:uint32,mem_length:uint32,compression_format:uint8}]
```

In the rest of this document, we will refer to this type as `<segmap>` for
shorthand and refer to the concept as a "segmap".

{{% tip "Note" %}}

We use the type name "segmap" to emphasize that this information represents
a set of byte ranges where data is stored and must be read from *rather than*
the data itself.

{{% /tip %}}

#### The Super Column

The first of the N+1 reassembly records defines the "super column", where this column
represents the sequence of [super types](#the-super-types) of each original value, i.e., indicating
which super type's column stream to select from to pull column values to form
the reconstructed value.
The sequence of super types is defined by each type's super ID (as defined above),
0 to N-1, within the set of N super types.

The super column stream is encoded as a sequence of Super Binary-encoded `int32` primitive values.
While there are a large number of entries in the super column (one for each original row),
the cardinality of super IDs is small in practice so this column
will compress very significantly, e.g., in the special case that all the
values in the Super Columnar file have the same super ID,
the super column will compress trivially.

The reassembly map appears as the next value in the reassembly section
and is of type `<segmap>`.

#### The Reassembly Records

Following the root reassembly map are N reassembly maps, one for each unique super type.

Each reassembly record is a record of type `<any_column>`, as defined below,
where each reassembly record appears in the same sequence as the original N schemas.
Note that there is no "any" type in the super data model, but rather this terminology is used
here to refer to any of the concrete type structures that would appear
in a given Super Columnar file.

In other words, the reassembly record of the super column
combined with the N reassembly records collectively define the original sequence
of data values in the original order.
Taken in pieces, the reassembly records allow efficient access to sub-ranges of the
rows, to subsets of columns of the rows, to sub-ranges of columns of the rows, and so forth.

This simple top-down arrangement, along with the definition of the other
column structures below, is all that is needed to reconstruct all of the
original data.

{{% tip "Note" %}}

Each row reassembly record has its own layout of columnar
values and there is no attempt made to store like-typed columns from different
schemas in the same physical column.

{{% /tip %}}

The notation `<any_column>` refers to any instance of the five column types:
* [`<record_column>`](#record-column),
* [`<array_column>`](#array-column),
* [`<union_column>`](#union-column),
* [`<map_column>`](#map-column), or
* [`<primitive_column>`](#primitive-column).

Note that when decoding a column, all type information is known
from the super type in question so there is no need
to encode the type information again in the reassembly record.

#### Record Column

A `<record_column>` is defined recursively in terms of the column types of
its fields, i.e., other types that represent arrays, unions, or primitive types
and has the form:
```
{
        <fld1>:{column:<any_column>,presence:<segmap>},
        <fld2>:{column:<any_column>,presence:<segmap>},
        ...
        <fldn>:{column:<any_column>,presence:<segmap>}
}
```
where
* `<fld1>` through `<fldn>` are the names of the top-level fields of the
original row record,
* the `column` fields are column stream definitions for each field, and
* the [`presence` columns](#presence-columns) are `int32` Super Binary column streams comprised of a
run-length encoding of the locations of column values in their respective rows,
when there are null values.

If there are no null values, then the `presence` field contains an empty `<segmap>`.
If all of the values are null, then the `column` field is null (and the `presence`
contains an empty `<segmap>`).  For an empty `<segmap>`, there is no
corresponding data stored in the data section.  Since a `<segmap>` is an
array, an empty `<segmap>` is simply the empty array value `[]`.

#### Array Column

An `<array_column>` has the form:
```
{values:<any_column>,lengths:<segmap>}
```
where
* `values` represents a continuous sequence of values of the array elements
that are sliced into array values based on the length information, and
* `lengths` encodes an `int32` sequence of values that represent the length
of each array value.

The `<array_column>` structure is used for both arrays and sets.

#### Map Column

A `<map_column>` has the form:
```
{key:<any_column>,value:<any_column>}
```
where
* `key` encodes the column of map keys and
* `value` encodes the column of map values.

#### Union Column

A `<union_column>` has the form:
```
{columns:[<any_column>],tags:<segmap>}
```
where
* `columns` is an array containing the reassembly information for each tagged union value
in the same column order implied by the union type, and
* `tags` is a column of `int32` values where each subsequent value encodes
the tag of the union type indicating which column the value falls within.

{{% tip "TBD" %}}

Change code to conform to columns array instead of record{c0,c1,...}

{{% /tip %}}

The number of times each value of `tags` appears must equal the number of values
in each respective column.

#### Primitive Column

A `<primitive_column>` is a `<segmap>` that defines a column stream of
primitive values.

#### Presence Columns

The presence column is logically a sequence of booleans, one for each position
in the original column, indicating whether a value is null or present.
The number of values in the encoded column is equal to the number of values
present so that null values are not encoded.

Instead the presence column is encoded as a sequence of alternating runs.
First, the number of values present is encoded, then the number of values not present,
then the number of values present, and so forth.   These runs are then stored
as `int32` values in the presence column (which may be subject to further
compression based on segment compression).

### The Trailer

After the reassembly section is a Super Binary stream with a single record defining
the "trailer" of the Super Columnar file.  The trailer provides a magic field
indicating the file format, a version number,
the size of the segment threshold for decomposing segments into frames,
the size of the skew threshold for flushing all segments to storage when
the memory footprint roughly exceeds this threshold,
and an array of sizes in bytes of the sections of the Super Columnar file.

This type of this record has the format
```
{magic:string,type:string,version:int64,sections:[int64],meta:{skew_thresh:int64,segment_thresh:int64}}
```
The trailer can be efficiently found by scanning backward from the end of the
Super Columnar file to find a valid Super Binary stream containing a single record value
conforming to the above type.

## Decoding

To decode an entire Super Columnar file into rows, the trailer is read to find the sizes
of the sections, then the Super Binary stream of the reassembly section is read,
typically in its entirety.

Since this data structure is relatively small compared to all of the columnar
data in the file,
it will typically fit comfortably in memory and it can be very fast to scan the
entire reassembly structure for any purpose.

{{% tip "Example" %}}

For a given query, a "scan planner" could traverse all the
reassembly records to figure out which segments will be needed, then construct
an intelligent plan for reading the needed segments and attempt to read them
in mostly sequential order, which could serve as
an optimizing intermediary between any underlying storage API and the
Super Columnar decoding logic.

{{% /tip %}}

To decode the "next" row, its schema index is read from the root reassembly
column stream.

This schema index then determines which reassembly record to fetch
column values from.

The top-level reassembly fetches column values as a `<record_column>`.

For any `<record_column>`, a value from each field is read from each field's column,
accounting for the presence column indicating null,
and the results are encoded into the corresponding Super Binary record value using
type information from the corresponding schema.

For a `<primitive_column>` a value is determined by reading the next
value from its segmap.

For an `<array_column>`, a length is read from its `lengths` segmap as an `int32`
and that many values are read from its the `values` sub-column,
encoding the result as a Super Binary array value.

For a `<union_column>`, a value is read from its `tags` segmap
and that value is used to select the corresponding column stream
`c0`, `c1`, etc.  The value read is then encoded as a Super Binary union value
using the same tag within the union value.

## Examples

### Hello, world

Start with this [Super JSON](sup.md) file `hello.sup`:
```
{a:"hello",b:"world"}
{a:"goodnight",b:"gracie"}
```

To convert to Super Columnar format:
```
super -f csup hello.sup > hello.csup
```

Segments in the Super Columnar format would be laid out like this:
```
=== column for a
hello
goodnight
=== column for b
world
gracie
=== column for schema IDs
0
0
===
```
