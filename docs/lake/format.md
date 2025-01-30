---
weight: 2
title: Format
heading: Zed Lake Format
---

## _Status_

>This document is a rough draft and work in progress.  We plan to
soon bring it up to date with the current implementation and maintain it
as we add new capabilities to the system.

## Introduction

To support the client-facing [SuperDB data lake semantics](../commands/super-db.md#the-lake-model)
implemented by the [`super db` command](../commands/super-db.md), we are developing
an open specification for the Zed lake storage format described in this document.
As we make progress on the Zed lake model, we will update this document
as we go.

The Zed lake storage format is somewhat analagous the emerging
cloud table formats like [Iceberg](https://iceberg.apache.org/spec/),
but differs but differs in a fundamental way: there are no tables in a Zed lake.

On the contrary, we believe a better approach for organizing modern, eclectic
data is based on a type system rather than a collection of tables
and relational schemas.  Since relations, tables, schemas, data frames,
Parquet files, Avro files, JSON, CSV, XML, and so forth are all subsets of the
Zed's super-structured type system, a data lake based on Zed holds the promise
to provide a universal data representation for all of these different approaches to data.

Also, while we are not currently focused on building a SQL engine for the Zed lake,
it is most certainly possible to do so, as a Zed record type
[is analagous to](../formats/_index.md#2-a-super-structured-pattern)
a SQL table definition.  SQL tables can essentially be dynamically projected
via a table virtualization layer built on top of the Zed lake model.

All data and metadata in a Zed lake conforms to the Zed data model, which materially
simplifies development, test, introspection, and so forth.

## Cloud Object Model

Every data element in a Zed lake is either of two fundamental object types:
* a single-writer _immutable object_, or
* a multi-writer _transaction journal_.

### Immutable Objects

All imported data in a data pool is composed of immutable objects, which are organized
around a primary data object.  Each data object is composed of one or more immutable objects
all of which share a common, globally unique identifier,
which is referred to below generically as `<id>` below.

These identifiers are [KSUIDs](https://github.com/segmentio/ksuid).
The KSUID allocation scheme
provides a decentralized solution for creating globally unique IDs.
KSUIDs have embedded timestamps so the creation time of
any object named in this way can be derived.  Also, a simple lexicographic
sort of the KSUIDs results in a creation-time ordering (though this ordering
is not relied on for causal relationships since clock skew can violate
such an assumption).

> While a Zed lake is defined in terms of a cloud object store, it may also
> be realized on top of a file system, which provides a convenient means for
> local, small-scale deployments for test/debug workflows.  Thus, for simple use cases,
> the complexity of running an object-store service may be avoided.

#### Data Objects

A data object is created by a single writer using a globally unique name
with an embedded KSUID.

New objects are written in their entirety.  No updates, appends, or modifications
may be made once an object exists.  Given these semantics, any such object may be
trivially cached as neither its name nor content ever change.

Since the object's name is globally unique and the
resulting object is immutable, there is no possible write concurrency to manage
with respect to a given object.

A data object is composed of the primary data object stored as one or two objects
(for sequence and/or vector layout) and an optional seek index.

Data objects may be either in sequence form (i.e., ZNG) or vector form (i.e., VNG),
or both forms may be present as a query optimizer may choose to use whatever
representation is more efficient.
When both sequence and vector data objects are present, they must contain the same
underlying Zed data.

Immutable objects are named as follows:

|object type|name|
|-----------|----|
|vector data|`<pool-id>/data/<id>.csup`|
|sequence data|`<pool-id>/data/<id>.bsup`|
|sequence seek index|`<pool-id>/data/<id>-seek.bsup`|

`<id>` is the KSUID of the data object.

The seek index maps pool key values to seek offsets in the ZNG file thereby
allowing a scan to do a byte-range retrieval of the ZNG object when
processing only a subset of data.

> Note the VNG format allows individual vector segments to be read in isolation
> and the in-memory VNG representation supports random access so there is
> no need to have a seek index for the vector object.

#### Commit History

A branch's commit history is the definitive record of the evolution of data in
that pool in a transactionally consistent fashion.

Each commit object entry is identified with its `commit ID`.
Objects are immutable and uniquely named so there is never a concurrent write
condition.

The "add" and "commit" operations are transactionally stored
in a chain of commit objects.  Any number of adds (and deletes) may appear
in a commit object.  All of the operations that belong to a commit are
identified with a commit identifier (ID).

As each commit object points to its parent (except for the initial commit
in main), the collection of commit objects in a pool forms a tree.

Each commit object contains a sequence of _actions_:

* `Add` to add a data object reference to a pool,
* `Delete` to delete a data object reference from a pool,
* `Commit` for providing metadata about each commit.

The actions are not grouped directly by their commit ID but instead each
action serialization includes its commit ID.

The chain of commit objects starting at any commit and following
the parent pointers to the original commit is called the "commit log".
This log represents the definitive record of a branch's present
and historical content, and accessing its complete detail can provide
insights about data layout, provenance, history, and so forth.

### Transaction Journal

State that is mutable is built upon a transaction journal of immutable
collections of entries.  In this way, there are no objects in the
storage footprint that are ever modified.  Instead, the journal captures
changes and journal snapshots are used to provide synchronization points
for efficient access to the journal (so the entire journal need not be
read to create the current state) and old journal entries may be removed
based on retention policy.

The journal may be updated concurrently by multiple writers so concurrency
controls are included (see [Journal Concurrency Control](#journal-concurrency-control)
below) to provide atomic updates.

A journal entry simply contains actions that modify the visible "state" of
the pool by changing branch name to commit object mappings.  Note that
adding a commit object to a pool changes nothing until a branch pointer
is mutated to point at that object.

Each atomic journal commit object is a ZNG file numbered 1 to the end of journal (HEAD),
e.g., `1.bsup`, `2.bsup`, etc., each number corresponding to a journal ID.
The 0 value is reserved as the null journal ID.
The journal's TAIL begins at 1 and is increased as journal entries are purged.
Entries are added at the HEAD and removed from the TAIL.
Once created, a journal entry is never modified but it may be deleted and
never again allocated.
There may be 1 or more entries in each commit object.

Each journal entry implies a snapshot of the data in a pool.  A snapshot
is computed by applying the transactions in sequence from entry TAIL to
the journal entry in question, up to HEAD.  This gives the set of commit IDs
that comprise a snapshot.

The set of branch pointers in a pool is assembled at any point in the journal's history
by scanning a journal that includes ADD, UPDATE, and DELETE actions for the
mapping of a branch name to a commit object.  A timestamp is recorded in
each action to provide for time travel.

For efficiency, a journal entry's snapshot may be stored as a "cached snapshot"
alongside the journal entry.  This way, the snapshot at HEAD may be
efficiently computed by locating the most recent cached snapshot and scanning
forward to HEAD.

#### Journal Concurrency Control

To provide for atomic commits, a writer must be able to atomically update
the HEAD of the log.  There are three strategies for doing so.

First, if the cloud service offers "put-if-missing" semantics, then a writer
can simply read the HEAD file and use put-if-missing to write to the
journal at position HEAD+1.  If this fails because of a race, then the writer
can simply write at position HEAD+2 and so forth until it succeeds (and
then update the HEAD object).  Note that there can be a race in updating
HEAD, but HEAD is always less than or equal to the real end of journal,
and this condition can be self-corrected by probing for HEAD+1 whenever
the HEAD of the journal is accessed.

> Note that put-if-missing can be emulated on a local file system by opening
> a file for exclusive access and checking that it has zero length after
> a successful open.

Second, strong read/write ordering semantics (as exists in [Amazon S3](../integrations/amazon-s3.md))
can be used to implement transactional journal updates as follows:
* _TBD: this is worked out but needs to be written up_

Finally, since the above algorithm requires many round trips to the storage
system and such round trips can be tens of milliseconds, another approach
is to simply run a lock service as part of a cloud deployment that manages
a mutex lock for each pool's journal.

#### Configuration State

Configuration state describing a lake or pool is also stored in mutable objects.
Zed lakes simply use a commit journal to store configuration like the
list of pools and pool attributes.
Here, a generic interface to a commit journal manages any configuration
state simply as a key-value store of snapshots providing time travel over
the configuration history.

### Merge on Read

To support _sorted scans_,
data objects are store in a sorted order defined by the pool's sort key.
The sort key may be a composite key compised of primary, secondary, etc
component keys.

When the key range of objects overlap, they may be read in parallel
in merged in sorted order.
This is called the _merge scan_.

If many overlapping data objects arise, performing a merge scan
on every read can be inefficient.
This can arise when
many random data `load` operations involving perhaps "late" data
(e.g., the pool key is a timestamp and records with old timestamp values regularly
show up and need to be inserted into the past).  The data layout can become
fragmented and less efficient to scan, requiring a scan to merge data
from a potentially large number of different objects.

To solve this problem, the Zed lake format follows the
[LSM](https://en.wikipedia.org/wiki/Log-structured_merge-tree) design pattern.
Since records in each data object are stored in sorted order, a total order over
a collection of objects (e.g., the collection coming from a specific set of commits)
can be produced by executing a sorted scan and rewriting the results back to the pool
in a new commit.  In addition, the objects comprising the total order
do not overlap.  This is just the basic LSM algorithm at work.

### Object Naming

```
<lake-path>/
  lake.bsup
  pools/
    HEAD
    TAIL
    1.bsup
    2.bsup
    ...
  <pool-id-1>/
    branches/
      HEAD
      TAIL
      1.bsup
      2.bsup
      ...
    commits/
      <id1>.bsup
      <id2>.bsup
      ...
    data/
      <id1>.{zng,vng}
      <id2>.{zng,vng}
      ...
  <pool-id-2>/
  ...
```
