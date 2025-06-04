### Command

&emsp; **manage** &mdash; run regular maintenance on a database

### Synopsis

```
super db manage [options]
```

### Options

Additional options of the [db sub-command](db.md#options)

### Description

The `manage` command performs maintenance tasks on a database.

Currently the only supported task is _compaction_, which reduces fragmentation
by reading data objects in a pool and writing their contents back to large,
non-overlapping objects.

If the `-monitor` option is specified and the database is
[configured](db.md#database-connection)
via network connection, `super db manage` will run continuously and perform updates
as needed.  By default a check is performed once per minute to determine if
updates are necessary.  The `-interval` option may be used to specify an
alternate check frequency as a [duration](../super-sql/types/time.md).

If `-monitor` is not specified, a single maintenance pass is performed on the
database.

By default, maintenance tasks are performed on all pools in the database.  The
`-pool` option may be specified one or more times to limit maintenance tasks
to a subset of pools listed by name.

The output from `manage` provides a per-pool summary of the maintenance
performed, including a count of `objects_compacted`.

As an alternative to running `manage` as a separate command, the `-manage`
option is also available on the [`serve`](db-serve.md) command to have maintenance
tasks run at the specified interval by the service process.
