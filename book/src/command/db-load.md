### Command

&emsp; **load** &mdash; load data into database

### Synopsis

```
super db load [options] input [input ...]
```

### Options

TODO

Additional options of the [db sub-command](db.md#options)

### Description

The `load` command commits new data to a branch of a pool.

Run `super db load -h` for a list of command-line options.

Note that there is no need to define a schema or insert data into
a "table" as all super-structured data is _self describing_ and can be queried in a
schema-agnostic fashion.  Data of any _shape_ can be stored in any pool
and arbitrary data _shapes_ can coexist side by side.

As with [`super`](super.md),
the [input arguments](super.md#options) can be in
any [supported format](super.md#supported-formats) and
the input format is auto-detected if `-i` is not provided.  Likewise,
the inputs may be URLs, in which case, the `load` command streams
the data from a Web server or [S3](../dev/integrations/s3.md)
and into the database.

When data is loaded, it is broken up into objects of a target size determined
by the pool's `threshold` parameter (which defaults to 500MiB but can be configured
when the pool is created).  Each object is sorted by the [sort key](db.md#sort-key) but
a sequence of objects is not guaranteed to be globally sorted.  When lots
of small or unsorted commits occur, data can be fragmented.  The performance
impact of fragmentation can be eliminated by regularly [compacting](db-manage.md)
pools.

For example, this command
```
super db load sample1.json sample2.bsup sample3.sup
```
loads files of varying formats in a single commit to the working branch.

An alternative branch may be specified with a branch reference with the
`-use` option, i.e., `<pool>@<branch>`.  Supposing a branch
called `live` existed, data can be committed into this branch as follows:
```
super db load -use logs@live sample.bsup
```
Or, as mentioned above, you can set the default branch for the load command
via [`use`](db-use.md):
```
super db use logs@live
super db load sample.bsup
```
During a `load` operation, a commit is broken out into units called _data objects_
where a target object size is configured into the pool,
typically 100MB-1GB.  The records within each object are sorted by the sort key.
A data object is presumed by the implementation
to fit into the memory of an intake worker node
so that such a sort can be trivially accomplished.

Data added to a pool can arrive in any order with respect to its sort key.
While each object is sorted before it is written,
the collection of objects is generally not sorted.

Each load operation creates a single [commit](../database/intro.md#commit-objects),
which includes:
* an author and message string,
* a timestamp computed by the server, and
* an optional metadata field of any type expressed as a Super (SUP) value.
This data has the type signature:
```
{
    author: string,
    date: time,
    message: string,
    meta: <any>
}
```
where `<any>` is the type of any optionally attached metadata .
For example, this command sets the `author` and `message` fields:
```
super db load -user user@example.com -message "new version of prod dataset" ...
```
If these fields are not specified, then the system will fill them in
with the user obtained from the session and a message that is descriptive
of the action.

The `date` field here is used by the database for
[time travel](../database/intro.md#time-travel)
through the branch and pool history, allowing you to see the state of
branches at any time in their commit history.

Arbitrary metadata expressed as any [SUP value](../formats/sup.md)
may be attached to a commit via the `-meta` flag.  This allows an application
or user to transactionally commit metadata alongside committed data for any
purpose.  This approach allows external applications to implement arbitrary
data provenance and audit capabilities by embedding custom metadata in the
commit history.

Since commit objects are stored as super-structured data, the metadata can easily be
queried by running the `log -f bsup` to retrieve the log in BSUP format,
for example, and using [`super`](super.md) to pull the metadata out
as in:
```
super db log -f bsup | super -c 'has(meta) | values {id,meta}' -
```
